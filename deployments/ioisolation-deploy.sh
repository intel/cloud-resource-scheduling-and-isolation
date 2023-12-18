#!/bin/bash
# Copyright (C) 2025 Intel Corporation
# SPDX-License-Identifier: Apache-2.0

# ./deployments/ioisolation-deploy.sh 127.0.0.1:5000 latest disk
REPO=$1
TAG=$2
RESOURCE=$3
# check input params
if [ "$#" -ne 3 ]; then
    echo "Usage: $0 <repo> <tag> <resource>"
    exit 1
fi

# check if yq is installed
if ! sudo bash -c 'command -v yq &> /dev/null'; then
    echo "yq could not be found, please install it first."
    exit 1
fi

install_scheduler() {
    FILE="/etc/kubernetes/manifests/kube-scheduler.yaml"
    SCHED_IMAGE=$REPO/ioisolation/kube-scheduler:$TAG

    if `sudo yq '.spec.containers[0].image == "'"${SCHED_IMAGE}"'"' $FILE`
    then
        echo "scheduler deployments deployed"
        return
    fi
    resourceType="All"
    if [ "$RESOURCE" == "disk" ]; then
        resourceType="BlockIO"
    elif [ "$RESOURCE" == "net" ]; then
        resourceType="NetworkIO"
    elif [ "$RESOURCE" == "rdt" ]; then
        resourceType="RDT"
    fi
    sudo yq -i '.profiles[0].pluginConfig[0].args.resourceType = "'"${resourceType}"'"' ./deployments/sched-cc.yaml  -y
    sudo cp ./deployments/sched-cc.yaml /etc/kubernetes/sched-cc.yaml
    sudo cp /etc/kubernetes/manifests/kube-scheduler.yaml /etc/kubernetes/kube-scheduler.yaml

    sudo yq -i '.spec.containers[0].command += ["--v=5"]'  $FILE -y
    sudo yq -i '.spec.containers[0].command += ["--config=/etc/kubernetes/sched-cc.yaml"]'  $FILE -y
    sudo yq -i '.spec.containers[0].image = "'"${SCHED_IMAGE}"'"'  $FILE -y
    sudo yq -i '.spec.containers[0].volumeMounts += [{"mountPath": "/etc/kubernetes/sched-cc.yaml", "name": "sched-cc", "readOnly": true}]'  $FILE -y
    sudo yq -i '.spec.volumes += [{"hostPath": {"path": "/etc/kubernetes/sched-cc.yaml", "type": "FileOrCreate"}, "name": "sched-cc"}]'  $FILE -y
}

label_non_control_plane_nodes() {
  nodes=$(kubectl get nodes --no-headers -o custom-columns=":metadata.name")
  for node in $nodes; do
    if kubectl get node $node -o jsonpath='{.metadata.labels}' | grep -q 'node-role.kubernetes.io/control-plane'; then
      echo "Skipping control-plane node: $node"
    else
      echo "Labeling node: $node"
      kubectl label nodes $node ioisolation=${RESOURCE} --overwrite
      # kubectl label nodes $node csi-seperated=true --overwrite
    fi
  done
}

# generate root CA
./hack/gen_rootca.sh skip

# deploy ioi-system namespace and node labels
kubectl create namespace ioi-system
label_non_control_plane_nodes
make deploy
# deploy scheduler
install_scheduler
# kubectl diskio tool installation
sudo chmod +x ./bin/kubectl-diskio
sudo cp ./bin/kubectl-diskio /usr/local/bin

# helm installation
helm install ioisolation ./deployments/helm-charts/ioisolation --namespace ioi-system --create-namespace --set image.repository=$REPO --set image.tag=$TAG
