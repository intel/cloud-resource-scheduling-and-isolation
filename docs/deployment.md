# IOIsolation deployment and Usage in Kubernetes


## 1. Environment Settings Recommendations
To make our project work for your solution, please do the environment settings following the best practices that are widely known by the communities and industries.
### 1.1 Hardware requirements
Please note that this software can only run on Intel Xeon platforms, and no testing has been conducted on other platforms.
if you want to use RDT(Resource Director Technology) module, please use follow command to check if the CPU can support RDT.
```
lscpu | grep -i rdt
```

### 1.1 Hardware
Please note that this solution excluding the RDT part has been conducted on Intel Xeon platforms and Intel Core platforms. 
But, We don't test it on other platforms.
If you want to use RDT(Resource Director Technology) module, please use follow command to check if the CPU can support RDT.
```
lscpu | grep -i rdt
```
The RDT was initially introduced in Intel's Xeon processor series, particularly targeting data center and server product lines.
We just verify RDT functions in follow cpu models:
```
Intel(R) Xeon(R) Gold 6140M CPU
Intel(R) Xeon(R) Gold 6348 CPU @ 2.60GHz
Intel(R) Xeon(R) Platinum 8280M CPU
```

### 1.2 Host OS
First of all, please follow the best practices to configure your host operating system. e.g. keep host OS components up-to-date and minimize host OS attack surface.

### 1.3 Kubernetes
Please follow the industry best practices for setting your Kubernetes clusters.
- [Kubernetes Security Tutorial](https://kubernetes.io/docs/tutorials/security/)
- [Kubernetes CIS Benchmark](https://www.aquasec.com/cloud-native-academy/kubernetes-in-production/kubernetes-cis-benchmark-best-practices-in-brief/)
- [Kube Bench](https://github.com/aquasecurity/kube-bench)
And follow the best practices of industry to manange your certificate. e.g. https://kubernetes.io/docs/setup/best-practices/certificates/

## 2 Administration/Operation Guideline
Please follow the best practices for administration or operations.

### 2.1 Certificate Generation and Management
The NodeIO Certificate is used for enabling mTLS communication between Node IO Isolation Agent and Node IO Isolation Service. Please apply your NodeIO Certificate from a well-known CA such as Sectigo, SSL.com, DigiCert, GlobalSign, etc. based on RSA algorithm with key size no smaller than 3072. 
The certificate should be stored under /etc/ioi/service/pki or other positions with read/write right only granted for root user.

### 2.2 Key Management
Please follow the best known practices of industries to manage your keys. e.g. Recommendation for Key Management [NIST SP 800-57 Part 1 Rev. 5](https://nvlpubs.nist.gov/nistpubs/SpecialPublications/NIST.SP.800-57pt1r5.pdf)
Following the standard requirement, the renew process should start when the certificate has passed 2/3 of its lifetime.

| Key Type  | Originator Usage Period (OUP) |
| ------------- | ------------- |
| Private Signature Key  | 1-3 years  |
| Public Signature Key  | Several years (depending on key size)  |
| Private Authentication Key  | 1-2 years  |
| Public Authentication Key  | 1-2 years  |
| Private Key Transport Key  | <2 years  |
| Public Key Transport Key  | 1-2 years  |

## 3. Prerequisites
### 3.1 Component Versions
The project is tested and developed with the cloud native components of the following versions. It is **highly recommended** to use these specified versions.    
| Component | Version | 
| ------------- | ------------- |
| Kubernetes | 1.29.9 |
| Containerd | 1.7.0 |
| Runc | 1.1.4 |
| Ubuntu | 22.04 |
| Go | 1.23.0 | 
| openssl | 3.0.2 | 
| Cgroup | V2 |

### 3.2. Limitation
#### 3.2.1  Disk I/O Isolation
* We don't support LVM when profiling the I/O bandwidth of disks.
* I/O resource request/limit specification only supports `emptyDir` typed and `PersistentVolumeClaim` typed volumes, the specifications for other types of volumes will be ignored. 
* A pod's block I/O resource's specified I/O block size for all volumes in pod annotation must keep consistent.

## 4. Configuration 
On **the worker nodes which would enable ioisolation feature**, enable NRI in containerd and generate a CA and self-signed certificates for ioi service and metrics server 
### 4.1 Install Dependencies 
Execute the following command to install build dependencies
```
sudo apt install openssl git python3-pip
```

### 4.2 [Optional]Enable NRI in Containerd
If you don't enable NRI, you will auto get data from kubelet.

```
1. download containerd binary
$ wget https://github.com/containerd/containerd/releases/download/v1.7.0/containerd-1.7.0-linux-amd64.tar.gz

2. stop running containerd
$ systemctl stop containerd

3. replace old containerd
$ sudo tar Cxzvf /usr/local containerd-1.7.0-linux-amd64.tar.gz

4. enable NRI in containerd
add an item in /etc/containerd/config.toml
[plugins."io.containerd.nri.v1.nri"]
    config_file = "/etc/nri/nri.conf"
    disable = false
    plugin_path = "/opt/nri/plugins"
    socket_path = "/var/run/nri/nri.sock"

5. add a new nri config in /etc/nri/nri.conf
disableConnections: false

6. restart containerd
$ sudo systemctl restart containerd
```

### 4.3 Ensuring time synchronization across the cluster

Make sure to sychronize the time across the cluster, otherwise the mutual tls may not function properly. You may refer to this [guide](https://ubuntu.com/server/docs/use-timedatectl-and-timesyncd) if you use Ubuntu.

### 4.4 [Network IO Isolation Only] CNI Requirement

Here are CNI configurations supported by network IO isolation. Replace `Master Node IP` and `Interface Name` by the actual value :

Note: If your worker nodes are using ADQ enabled E810 Intel Ethernet Network Adapter, to enable hardware-based network IO bandwidth throttling, choose CNI configurations, only `Cilium Veth Mode` and `Calico BGP mode` are supported currently.

1. Cilium Veth Mode
    ```
    # kubectl apply -f deployments/cilium-cm.yaml
    # helm uninstall -n kube-system cilium
    # helm install cilium cilium/cilium \
    --version v1.14.0 \
    --namespace kube-system \
    --set kubeProxyReplacement=strict \
    --set k8sServiceHost=<Master Node IP>   \
    --set k8sServicePort=6443 \
    --set devices=<Interface Name>  \
    --set l7Proxy=false \
    --set sockops.enabled=true \
    --set tunnel=disabled \
    --set ipv4NativeRoutingCIDR=10.244.0.0/16 \
    --set enableipv4masquerade=true \
    --set autoDirectNodeRoutes=true \
    --set endpointRoutes.enabled=true \
    --set bpf.masquerade=true \
    --set ipv4.enabled=true \
    --set disable-envoy-version-check=true \
    --set ipam.mode=kubernetes \
    --set cni.customConf=true \
    --set cni.configMap=cni-configuration \
    --set prometheus.enabled=true \
    --set operator.prometheus.enabled=true \
    --set hubble.enabled=true \
    --set hubble.metrics.enabled="{dns,drop,tcp,flow,port-distribution,icmp,http}" \
    --set extraArgs='{--bpf-filter-priority=99}'  
    ```
2. Calico BGP mode
   ```
   # wget https://docs.projectcalico.org/manifests/calico.yaml
   ```
   Change ***CALICO_IPV4POOL_IPIP*** from "Always" to "Never". 
   Install ***calicoctl***.
   ```
   # kubectl apply -f  calico.yaml
   # calicoctl patch felixconfiguration default --patch='{"spec": {"bpfEnabled": true}}'
   ```

3. Calico VXLan mode
   Install the operator on your cluster.
   ```
   # kubectl create -f https://raw.githubusercontent.com/projectcalico/calico/v3.28.0/manifests/tigera-operator.yaml
   ```

   Download the custom resources necessary to configure Calico.
   ```
   # curl https://raw.githubusercontent.com/projectcalico/calico/v3.28.0/manifests/custom-resources.yaml -O
   ```

   Change ***encapsulation*** to "VXLAN" in custom-resources.yaml and Create the manifest to install Calico.
   ```
   # kubectl create -f custom-resources.yaml
   ```

#### [Optional] How to enable ADQ in network IO throttling?

Only for E810 Intel Ethernet Network Adapter, otherwise please skip this step.

Download configuration guide from: https://www.intel.com/content/www/us/en/support/articles/000088449/ethernet-products/800-series-network-adapters-up-to-100gbe.html

Before deploying ioisolation components, you need tp deploy adqsetup in each worker node, the script hack/auto_setup.sh helps to automatically install and patch the dependency adqsetup.
Or you can also install adqsetup and apply the patch deployments/adqsetup_ioi.diff manually in each worker node.

```
pip3 install adqsetup==2.0.2
adqsetuppath=`pip3 show  adqsetup | grep -E "Location" | cut -d ' ' -f 2-`
cp deployments/adqsetup_ioi.diff $adqsetuppath
cd $adqsetuppath
patch -p1 < adqsetup_ioi.diff
```

### 4.5 [RDT Isolation Only] enable RDT
If you want to use RDT service, you need to enable RDT on the node.

Mount resctrl to the directory `/sys/fs/resctrl`:
```
sudo mount -t resctrl resctrl /sys/fs/resctrl
```
Modify containerd service's `/etc/containerd/config.toml` and set `rdt_config_file` to the path of the RDT config file, and restart containerd.
```
sudo systemctl restart containerd.service 
sudo systemctl status containerd.service 
```
RDT config file example:
```
options:
  l2:
    optional: true
  l3:
    optional: true
  mb:
    optional: true
partitions:
  default:
    l3Allocation: "100%"
    mbAllocation: ["100%"]
    classes:
      guaranteed:
        l3Allocation: "100%"
        mbAllocation: ["100%"]
      burstable:
        l3Allocation: "50%"
        mbAllocation: ["100%"]
      besteffort:
        l3Allocation: "50%"
        mbAllocation: ["100%"]
```
If you configure the RDT config file according to the example above, then your `/sys/fs/resctrl` directory should contain three folders, namely: guaranteed, burstable and besteffort. These three folders represent three RDT groups. You can verify that the schemata of these three groups are consistent with the RDT config file.

## 5. Prepare images

Compile and make images of kube-scheduler, aggregator, node-agent, localstorage-csi and node agent's init container. Put them to your image repositry.
```
$ make
$ make image REPO_HOST=<your-image-repo>  eg: 127.0.0.1:5000
$ make push_to_repo REPO_HOST=<your-image-repo>  eg: 127.0.0.1:5000
```

## 6. Deployment

If this is the first time you deploy ioisolation feature in your cluster, you can deploy the step 6 by running an auto deploy script.
Make sure to install `yq` in your root user before running the script:
```
# pip3 install yq==3.4.3
```
namespaceWhitelist is the list for the namespace in which pods can bypass IO scheduling and isolation. We offering `ioi-system`, `kube-system` and `kube-flannel` as default and it's also editable in `deployments/helm-charts/ioisolation/values.yaml`.
```
$ make
$ ./deployments/ioisolation-deploy.sh <Repo> <Image tag> <resource(refer to 6.1.2)>
For example
$ ./deployments/ioisolation-deploy.sh 127.0.0.1:5000 release-v0.8.0 disk
```

### 6.1 Configuration
#### 6.1.1 Init
if want to change deploy image, you can execute below command.
```
$ make change_version REPO_HOST=<your-image-repo>  eg: 127.0.0.1:5000
```
#### 6.1.2 Set node label
The worker node labeled with **ioisolation** would be temporarily unavailable for scheduling until the node agent completes profiling.
Character '-' can be used to combine multiple resources and "all" for all resources.

For disk I/O
```
$ kubectl label nodes <Node Name> ioisolation=disk
```
For network I/O
```
$ kubectl label nodes <Node Name> ioisolation=net
```
For RDT
```
$ kubectl label nodes <Node Name> ioisolation=rdt
```
For disk and network
```
$ kubectl label nodes <Node Name> ioisolation=net-disk
```
For all
```
$ kubectl label nodes <Node Name> ioisolation=all
```

#### 6.1.3 Create namespace

```
$ kubectl create namespace ioi-system
```

#### 6.1.4 Apply configmaps
`admin-configmap.yaml` is the admin config for node agent, including the following settings. MAKE SURE to edit it with your environment information:
diskpools and netpools are the proportions of each Qos type in resource pools. They are editable according to the pool size you want.
namespaceWhitelist is the list for the namespace in which pods can bypass the scheduling. We offering `ioi-system`, `kube-system` and `kube-flannel` as default and it's also editable.
loglevel is used to control all component log level. node field should be the common substring of specifie node names, and level field can be warn, info or debug. 
e.g.: if you want to debug, then set node1, node2 log to debug level, you can set like below.
```
  loglevel: |
    nodes=node
    level=debug
```
e.g.: if you want to in product, you can set node1, node2 log to warn or don't set, the default value is warn, then just print warn and error log
```
  loglevel: |
    nodes=node
    level=warn
```

disks is the all disks in the node which want to profile. you must set it according your environment.
diskRatio is ratio which reserve to system capacity size.

`policy-configmap.yaml` is the policy used when setting limits to pods. At present the supported policy is Default and more policies will be support in the future. It's editable according to the policy you want.

```
$ kubectl apply -f ./deployments/admin-configmap.yaml
$ kubectl apply -f ./deployments/policy-configmap.yaml
```

#### 6.1.5 [Block IO Isolation Only] Register CSI driver and Storage class
Apply CSI driver and storage class.
```
$ kubectl apply -f deployments/csi-driver.yaml
$ kubectl apply -f deployments/storageclass.yaml
```
#### 6.1.6 Generate the self-signed CA in master node
On **the master node**, generate the self-signed CA if a trusted root CA is not provided.  
Run the script below to generate the root CA and store it in k8s secret ：
```
./hack/gen_rootca.sh
```

### 6.2 Install scheduler
#### 5.2.1 Apply CRDs and CRD's RBAC to Kube-scheduler

```
$ make deploy
$ kubectl get crd nodestaticioinfoes.ioi.intel.com
```

The scheduler should be configured on **the master node**. 
#### 6.2.2 Create kube-scheduler configuration file.

```
sudo cp deployments/sched-cc.yaml /etc/kubernetes/sched-cc.yaml
```
Edit /etc/kubernetes/sched-cc.yaml line #17
Only enable block io resource's scheduling feature:
```
resourceType: BlockIO
```
Only enable network io resource's scheduling feature:
```
resourceType: NetworkIO
```
Only enable rdt resource's scheduling feature:
```
resourceType: RDT
```
Enable both block io resource's scheduling and network io resource's scheduling feature:
```
resourceType: All
```

#### 6.2.3 Modify `/etc/kubernetes/manifests/kube-scheduler.yaml` to run scheduler-plugins with IOIsolation.

Before replacing kube-scheduler, backup kube-scheduler.yaml.
```
sudo cp /etc/kubernetes/manifests/kube-scheduler.yaml /etc/kubernetes/kube-scheduler.yaml
```

Replace scheduler container image, set I/O Isolation scheduler plugin's configuration file `/etc/kubernetes/sched-cc.yaml` as config parameter and mount the following volumes to scheduler container. Replace `127.0.0.1:5000` with your local registry.
**Please note that the line numbers below may change, so please refer to the reference code for accuracy.**
```
18,20c18,20
<     - --leader-elect=true
<     image: registry.k8s.io/kube-scheduler:v1.28.4
---
>     - --v=4
>     - --config=/etc/kubernetes/sched-cc.yaml
>     image: 127.0.0.1:5000/ioisolation/kube-scheduler:latest
49a50,57
      volumeMounts:
      - mountPath: /etc/kubernetes/scheduler.conf
        name: kubeconfig
        readOnly: true
>     - mountPath: /etc/kubernetes/sched-cc.yaml
>       name: sched-cc
>       readOnly: true
59a66,77
    - hostPath:
        path: /etc/kubernetes/scheduler.conf
        type: FileOrCreate
      name: kubeconfig
>   - hostPath:
>       path: /etc/kubernetes/sched-cc.yaml
>       type: FileOrCreate
>     name: sched-cc
```

Then the scheduler running in your cluster is replaced by our customized one with the ResourceIO plugin.

### 6.3. Deploy Aggregator 
#### 6.3.1 Apply rbac for aggregator
```
$ kubectl apply -f ./deployments/rbac/aggregator-clusterrole.yaml
$ kubectl apply -f ./deployments/rbac/aggregator-service-account.yaml
$ kubectl apply -f ./deployments/rbac/aggregator-clusterrolebinding.yaml
```

#### 6.3.2 Deploy aggregator service
```
$ kubectl apply -f ./deployments/aggregator-service.yaml
```

### 6.4. Install node agent
On **the worker nodes labeled with `ioisolation`**, start the node agent. For block IO Isolation enabled case, if you want to run local storage csi outside node agent, label the node via:
```
kubectl label nodes <Node Name> csi-seperated=true
```
Otherwise, csi driver will be running inside node agents by default.

#### 6.4.1 Apply rbac for node agent

```
$ kubectl apply -f ./deployments/rbac/agent-clusterrole.yaml
$ kubectl apply -f ./deployments/rbac/agent-service-account.yaml
$ kubectl apply -f ./deployments/rbac/agent-clusterrolebinding.yaml
```
#### 6.4.2 Apply node agent and service
##### 6.4.2.1 one key delpoy node agent and service

```
$ kubectl apply -f ./deployments/node-agent-daemonset.yaml
```

##### 6.4.2.2 split node agent and service in two command
in master node

```
$ kubectl apply -f ./deployments/node-agent-daemonset-without-init.yaml
```

in every node
```
sudo cp ./deployments/ioi-service.toml /etc/ioi-service.toml
sudo mv ./bin/ioi-service /usr/local/bin 
sudo mv ./deployments/ioi-service.service /usr/lib/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now ioi-service
sudo systemctl restart ioi-service
sudo systemctl status ioi-service
```

##### 6.4.3 check static cr
**Note:** Node agent would start a go routine to run disk profiling which takes round 10 minutes.Begin to test the ioisolation feature **only when** the profile completes. Otherwise, pods would not be scheduled onto the node.  The following commands help to check if the disk profiling is completed.

```
$ kubectl get nodestaticioinfoes.ioi.intel.com -n ioi-system
NAME                         AGE
ioi-1-nodestaticioinfo   3h29m

$ kubectl get nodestaticioinfoes.ioi.intel.com/ioi-1-nodestaticioinfo -n ioi-system --output="jsonpath={.spec.resourceConfig}"
{"BlockIO":{"bePool":20,"devices":{"xxx":{"capacityIn":1589,"capacityOut":851.2,"capacityTotal":963.2,"defaultIn":5,"defaultOut":5,"diskSize":"378Gi","name":"/dev/nvme0n1","readRatio":{"16k":1.08,"1k":6.41,"32k":1,"4k":1.75,"512":13.43,"8k":1.14},"type":"default","writeRatio":{"16k":1.06,"1k":28,"32k":1,"4k":5.09,"512":53.2,"8k":1.99}},"Virtual_disk":{"capacityIn":2426,"capacityOut":2093,"capacityTotal":2325,"defaultIn":5,"defaultOut":5,"diskSize":"26Gi","name":"/dev/sda","readRatio":{"16k":1.29,"1k":16.07,"32k":1,"4k":3.83,"512":35.68,"8k":2.23},"type":"others","writeRatio":{"16k":1.63,"1k":261.62,"32k":1,"4k":12.61,"512":697.67,"8k":4.57}}},"gaPool":100}}
```
Replace the `ioi-1-nodestaticioinfo` with the actual CR name in the second command. The profile completes when the CR contains `BlockIO` or `NetworkIO` info. 

#### 6.5 [Block IO Isolation Only] Apply local storage csi plugin if you want to run local storage csi seperately
#### 6.5.1 Apply rbac for local storage csi
```
kubectl apply -f deployments/rbac/localstorage-csi.yaml
```
#### 6.5.2 Apply local storage csi if local-storage csi is running outside node agent.
```
kubectl apply -f deployments/localstorage-csi.yaml
```
#### 6.6 [Block IO Isolation Only] Install diskio kubectl plugin to add/delete disk dynamically

```
sudo chmod +x bin/kubectl-diskio
sudo mv bin/kubectl-diskio /usr/local/bin
```

## 7. Usage
Pod's I/O bandwidth request/limit and its QoS are specified in pod's annotation, in json format, if the pod's I/O requested/limited resources are not specified in annotation, it is treated as `Best Effort` type pod when running in an IOI-enabled node.

### 7.1 Disk I/O example:
```
apiVersion: v1
kind: Pod
metadata:
  name: sample-diskio-pod
  annotations:
    blockio.kubernetes.io/resources: "{\"volume1\":{\"requests\":{\"rbps\":\"50Mi/4k\",\"wbps\":\"50Mi/4k\"},\"limits\":{\"rbps\":\"50Mi/4k\",\"wbps\":\"50Mi/4k\"}},\"volume2\":{\"requests\":{\"rbps\":\"50Mi/4k\",\"wbps\":\"50Mi/4k\"},\"limits\":{\"rbps\":\"50Mi/4k\",\"wbps\":\"50Mi/4k\"}}}"
spec:
...
    volumeMounts:
    - mountPath: /mnt
      name: volume1
    volumeMounts:
    - mountPath: /opt
      name: volume2
  volumes:
  - name: volume1
    emptyDir: {}
  - name: volume2
    persistentVolumeClaim:
      claimName: pvc1
```
The pod's requested and limited block I/O resources of `PersistentVolumeClaim` typed and `emptyDir` typed volume can be specified in the pod annotation, and all those volumes' I/O QoS must be consistent for the same pod. "50Mi/4k" means the request/limit disk i/o bandwidth is set to 50MBps in 4k block size. The block size values should be same within a same pod. 
To use `PersistentVolumeClaim` typed volume, make sure that the PVC's storage class provisioner is `localstorage.csi.k8s.io` and the PVC's `accessModes` is `ReadWriteOncePod`, you can use the deployed storage class in `deployments/storageclass.yaml`. An example of a PVC definition:
```
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: pvc-example
spec:
  accessModes:
  - ReadWriteOncePod
  resources:
    requests:
      storage: 1Gi
  storageClassName: csi-hostpath-sc
```

Disk IO Isolation supports dynamically add, delete and list disk in nodes via kubectl commands, which means scheduler will/will not schedule disk IO aware workloads to those disks, only disks in Ready status can schedule disk I/O aware pod to it:
```
Usage: kubectl diskio --op=<operation> --node=<node> --disk=<dev>
Operations: list, get, delete, help
```
Add a disk by:
```
kubectl diskio --op=add --node=worker-1 --disk=/dev/sda
```
Delete a disk by:
```
kubectl diskio --op=delete --node=worker-1 --disk=/dev/sda
```
list all disk status in a node by:
```
$ kubectl diskio --op=list --node=worker-1
        Device|  Status|  In Use|Reason
  /dev/nvme0n1|   Ready|   false|
      /dev/sda|   Ready|   false|
```
By default, only the disk which emptyDir filesystem allocated is in Ready status at first. Other disks can be added by users via kubectl command.

### 7.2 Network I/O example:
```
networkio.kubernetes.io/resources: "{\"requests\":{\"ingress\":\"50M\",\"egress\":\"50M\"},\"limits\":{\"ingress\":\"50M\",\"egress\":\"50M\"}}"
```
For `Guaranteed` type workload, both requests and limits values should be specified and be equal. For `Burstable` type workload, request must be specified and limit is optional. For `Best Effort` type workload, users don't need to add it into pod annotation.

### 7.3 RDT I/O example:

#### 7.3.1 RDT annotation
```
rdt.resources.beta.kubernetes.io/pod: "guaranteed"
```

This annotation means that this pod belongs to guaranteed RDT class. It should be noted that the RDT class of a pod is guaranteed, which does not mean that the QoS of this pod is guaranteed. In other words, the RDT class and QoS are different.

#### 7.3.1 RDT config
RDT can dynamically adjust the L3 cache resources of the RDT classes based on the CPU utilization and l3CacheMiss of the RDT classes.

Users tell the RDT service how to make adjustments by configuring the `ioi-rdt-service.cfg` file.
Fie `./deployments/rdt/ioi-rdt-service.cfg` is an example. Users can modify it based on their own requirements.
