package dns

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/kubernetes/pkg/api"
	kcoreclient "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/core/internalversion"
	kcoreinformers "k8s.io/kubernetes/pkg/client/informers/informers_generated/internalversion/core/internalversion"
)

// ServiceAccessor is the interface used by the ServiceResolver to access
// services.
type ServiceAccessor interface {
	kcoreclient.ServicesGetter
	ServiceByClusterIP(ip string) (*api.Service, error)
}

// cachedServiceAccessor provides a cache of services that can answer queries
// about service lookups efficiently.
type cachedServiceAccessor struct {
	store cache.Indexer
}

// cachedServiceAccessor implements ServiceAccessor
var _ ServiceAccessor = &cachedServiceAccessor{}

// NewCachedServiceAccessor returns a service accessor that can answer queries about services.
// It uses a backing cache to make ClusterIP lookups efficient.
func NewCachedServiceAccessor(serviceInformer kcoreinformers.ServiceInformer) (ServiceAccessor, error) {
	if _, found := serviceInformer.Informer().GetIndexer().GetIndexers()["namespace"]; !found {
		err := serviceInformer.Informer().AddIndexers(cache.Indexers{
			"namespace": cache.MetaNamespaceIndexFunc,
		})
		if err != nil {
			return nil, err
		}
	}
	err := serviceInformer.Informer().AddIndexers(cache.Indexers{
		"clusterIP": indexServiceByClusterIP, // for reverse lookups
	})
	if err != nil {
		return nil, err
	}
	return &cachedServiceAccessor{store: serviceInformer.Informer().GetIndexer()}, nil
}

// ServiceByClusterIP returns the first service that matches the provided clusterIP value.
// errors.IsNotFound(err) will be true if no such service exists.
func (a *cachedServiceAccessor) ServiceByClusterIP(ip string) (*api.Service, error) {
	items, err := a.store.Index("clusterIP", &api.Service{Spec: api.ServiceSpec{ClusterIP: ip}})
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, errors.NewNotFound(api.Resource("service"), "clusterIP="+ip)
	}
	return items[0].(*api.Service), nil
}

// indexServiceByClusterIP creates an index between a clusterIP and the service that
// uses it.
func indexServiceByClusterIP(obj interface{}) ([]string, error) {
	return []string{obj.(*api.Service).Spec.ClusterIP}, nil
}

func (a *cachedServiceAccessor) Services(namespace string) kcoreclient.ServiceInterface {
	return cachedServiceNamespacer{a, namespace}
}

// TODO: needs to be unified with Registry interfaces once that work is done.
type cachedServiceNamespacer struct {
	accessor  *cachedServiceAccessor
	namespace string
}

var _ kcoreclient.ServiceInterface = cachedServiceNamespacer{}

func (a cachedServiceNamespacer) Get(name string, options metav1.GetOptions) (*api.Service, error) {
	item, ok, err := a.accessor.store.Get(&api.Service{ObjectMeta: metav1.ObjectMeta{Namespace: a.namespace, Name: name}})
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.NewNotFound(api.Resource("service"), name)
	}
	return item.(*api.Service), nil
}

func (a cachedServiceNamespacer) List(options metav1.ListOptions) (*api.ServiceList, error) {
	if len(options.LabelSelector) > 0 {
		return nil, fmt.Errorf("label selection on the cache is not currently implemented")
	}
	items, err := a.accessor.store.Index("namespace", &api.Service{ObjectMeta: metav1.ObjectMeta{Namespace: a.namespace}})
	if err != nil {
		return nil, err
	}
	services := make([]api.Service, 0, len(items))
	for i := range items {
		services = append(services, *items[i].(*api.Service))
	}
	return &api.ServiceList{
		// TODO: set ResourceVersion so that we can make watch work.
		Items: services,
	}, nil
}

func (a cachedServiceNamespacer) Create(srv *api.Service) (*api.Service, error) {
	return nil, fmt.Errorf("not implemented")
}
func (a cachedServiceNamespacer) Update(srv *api.Service) (*api.Service, error) {
	return nil, fmt.Errorf("not implemented")
}
func (a cachedServiceNamespacer) UpdateStatus(srv *api.Service) (*api.Service, error) {
	return nil, fmt.Errorf("not implemented")
}
func (a cachedServiceNamespacer) Delete(name string, options *metav1.DeleteOptions) error {
	return fmt.Errorf("not implemented")
}
func (a cachedServiceNamespacer) DeleteCollection(options *metav1.DeleteOptions, listOptions metav1.ListOptions) error {
	return fmt.Errorf("not implemented")
}
func (a cachedServiceNamespacer) Watch(options metav1.ListOptions) (watch.Interface, error) {
	return nil, fmt.Errorf("not implemented")
}
func (a cachedServiceNamespacer) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (*api.Service, error) {
	return nil, fmt.Errorf("not implemented")
}
func (a cachedServiceNamespacer) ProxyGet(scheme, name, port, path string, params map[string]string) restclient.ResponseWrapper {
	return nil
}
