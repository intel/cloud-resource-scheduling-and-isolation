# IO Driver
The IO dirver is responsible for profiling nodes' disk capacity at initial phase, monitoring and reporting each disk’s normalized available IO capacity at runtime. The IO driver is supposed to be implemented by disk IO vendors. This is a reference implementation of IO driver. This implementation is developed for [disk IO aware scheduler](https://github.com/kubernetes-sigs/scheduler-plugins/tree/master/kep/624-disk-io-aware-scheduling). 

# Tutorial 
This IO driver reports a fake device to API server through NodeDiskDevice CR. Moreover, it caches the reserved pods on node and update node's available capacity by listing and watching the creation and updation of NodeDiskIOStats CR.
Here are the steps to deploy it. 
1. Build and upload the docker image. Replace `<Your-Corperate-Proxy>` and `<IMAGE REGISTRY>` with the actual value/
``` shell
HTTPS_PROXY=<Your-Corperate-Proxy> REPO_HOST=<IMAGE REGISTRY> make image 
REPO_HOST=<IMAGE REGISTRY> make push_image
```
2. Deploy IO driver in an existing k8s cluster.
``` shell
kubectl create ns ioi-system
kubectl apply -f manifests/cluster_role.yaml
kubectl apply -f manifests/daemonset.yaml
```
