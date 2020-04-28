# HybridApplication-operator

-----

## Overview

Hybrid Application operator, generate Application and HybridDeployables from ApplicationAssembler

## Quick Start

-----

- Clone Repository

```shell
mkdir -p "$GOPATH"/src/github.com/hybridapp-io
cd "$GOPATH"/srcgithub.com/hybridapp-io
git clone https://github.com/hybridapp-io/ham-application-assembler.git
cd "$GOPATH"/src/github.com/hybridapp-io/ham-application-assembler
```

- Install CRDs, Dependencies, Create Clusters and Cluster Namespaces (toronto, raleigh)

```shell
% kubectl apply -f ./deploy/crds/app.cp4mcm.ibm.com_hybriddeployables_crd.yaml
% kubectl apply -f ./hack/test
% kubectl apply -f ./hack/test/crs
```

This creates a `db2` deployable CR with `deployer-type` annotation in `raleigh` namespace

- Start Hybrid Application operator (in another terminal)
    - opertor-sdk must be installed
    - kubeconfig points to a kubernetes cluster with admin privliege

```shell
cd "$GOPATH"/src/github.com/hybridapp-io/ham-application-assembler
operator-sdk run --local --namespace=""
```

- Create ApplicationAssembler example and found Application and HybridDeployable resources are created.

```shell
% kubectl apply -f ./examples/simple.yaml
applicationassembler.app.cp4mcm.ibm.com/simple created
% kubectl get hdpls
NAME   AGE
db2    13s
% kubectl get hdpls -o yaml
apiVersion: v1
items:
- apiVersion: app.cp4mcm.ibm.com/v1alpha1
  kind: HybridDeployable
  metadata:
    creationTimestamp: "2020-02-05T22:07:42Z"
    generation: 1
    labels:
      app.cp4mcm.ibm.com/application-assembler-id: ee408e63-4863-11ea-9dba-00000a101bb8
    name: db2
    namespace: default
    resourceVersion: "13422517"
    selfLink: /apis/app.cp4mcm.ibm.com/v1alpha1/namespaces/default/hybriddeployables/db2
    uid: ee4efc8c-4863-11ea-9dba-00000a101bb8
  spec:
    hybridtemplates:
    - deployerType: kubernetes
      template:
        apiVersion: apps/v1
        kind: Deployment
        spec:
          replicas: 1
          selector:
            matchLabels:
              app: nginx
          template:
            metadata:
              labels:
                app: nginx
            spec:
              containers:
              - image: nginx:1.7.9
                name: nginx
                ports:
                - containerPort: 80
kind: List
metadata:
  resourceVersion: ""
  selfLink: ""
% kubectl get applications
NAME     AGE
simple   63s
% kubectl get applications -o yaml
apiVersion: v1
items:
- apiVersion: app.k8s.io/v1beta1
  kind: Application
  metadata:
    creationTimestamp: "2020-02-05T22:07:42Z"
    generation: 1
    labels:
      app.cp4mcm.ibm.com/application-assembler-id: ee408e63-4863-11ea-9dba-00000a101bb8
    name: simple
    namespace: default
    ownerReferences:
    - apiVersion: app.cp4mcm.ibm.com/v1alpha1
      blockOwnerDeletion: true
      controller: true
      kind: ApplicationAssembler
      name: simple
      uid: ee408e63-4863-11ea-9dba-00000a101bb8
    resourceVersion: "13422519"
    selfLink: /apis/app.k8s.io/v1beta1/namespaces/default/applications/simple
    uid: ee5fb0fe-4863-11ea-9dba-00000a101bb8
  spec:
    componentKinds:
    - group: app.cp4mcm.ibm.com
      kind: HybridDeployable
    descriptor: {}
    selector:
      matchLabels:
        app.cp4mcm.ibm.com/application-assembler-id: ee408e63-4863-11ea-9dba-00000a101bb8
  status: {}
kind: List
metadata:
  resourceVersion: ""
  selfLink: ""
```
