# Application Assembler

[![Build](http://prow.yellow-chesterfield.com/badge.svg?jobs=multiarch-image-ham-application-assembler-postsubmit)](http://prow.yellow-chesterfield.com/?job=multiarch-image-ham-application-assembler-postsubmit)
[![GoDoc](https://godoc.org/github.com/hybridapp-io/ham-application-assembler?status.svg)](https://godoc.org/github.com/hybridapp-io/ham-application-assembler)
[![Go Report Card](https://goreportcard.com/badge/github.com/hybridapp-io/ham-application-assembler)](https://goreportcard.com/report/github.com/hybridapp-io/ham-application-assembler)
[![Code Coverage](https://codecov.io/gh/hybridapp-io/ham-application-assembler/branch/master/graphs/badge.svg?branch=master)](https://codecov.io/gh/hybridapp-io/ham-application-assembler?branch=master)
[![License](https://img.shields.io/:license-apache-blue.svg)](http://www.apache.org/licenses/LICENSE-2.0.html)
[![Docker Repository on Quay](https://quay.io/repository/cicdtest/ham-application-assembler/status?token=4b2b7d63-6560-46bf-a421-bec6ddc02a0f "Docker Repository on Quay")](https://quay.io/repository/cicdtest/ham-application-assembler)

<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**  *generated with [DocToc](https://github.com/thlorenz/doctoc)*

- [Application Assembler](#application-assembler)
  - [What is the Application Assembler](#what-is-the-application-assembler)
  - [Community, discussion, contribution, and support](#community-discussion-contribution-and-support)
  - [Getting Started](#getting-started)
    - [Prerequisites](#prerequisites)
    - [Quick Start](#quick-start)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

## What is the Application Assembler

Application Assembler generates Application, Deployables and PlacementRule

## Community, discussion, contribution, and support

Check the [CONTRIBUTING Doc](CONTRIBUTING.md) for how to contribute to the repo.

## Getting Started

### Prerequisites

- git v2.18+
- Go v1.13.4+
- operator-sdk v0.18.0+
- Kubernetes v1.14+
- kubectl v1.14+

Check the [Development Doc](docs/development.md) for how to contribute to the repo.

### Quick Start

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
% kubectl apply -f ./deploy/crds
% kubectl apply -f ./hack/test
% kubectl apply -f ./hack/test/crs
```

This creates a `db2` deployable CR with `deployer-type` annotation in `raleigh` namespace

- Start Application Assembler (in another terminal)
    - opertor-sdk must be installed

    - kubeconfig points to a kubernetes cluster with admin privliege

```shell
cd "$GOPATH"/src/github.com/hybridapp-io/ham-application-assembler
operator-sdk run --local --namespace=""
```

- Create ApplicationAssembler example and found Application and Deployable resources are created.

```shell
% kubectl apply -f ./examples/simple.yaml
applicationassembler.tools.hybridapp.io "simple" deleted
deployable.core.hybridapp.io "nginx" deleted
% kubectl get hdpl nginx -o yaml
apiVersion: core.hybridapp.io/v1alpha1
kind: Deployable
metadata:
  creationTimestamp: "2020-02-05T22:07:41Z"
  generation: 1
  labels:
    tools.hybridapp.io/application-868c6b0b-7e9a-4a96-8558-ca08bcb623c3: 868c6b0b-7e9a-4a96-8558-ca08bcb623c3
  name: nginx
  namespace: default
  resourceVersion: "13422517"
  selfLink: /apis/core.hybridapp.io/v1alpha1/namespaces/default/deployables/db2
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
% kubectl get applications -o yaml
apiVersion: app.k8s.io/v1beta1
kind: Application
metadata:
  creationTimestamp: "2020-02-05T23:08:42Z"
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
    uid: ee408e63-4863-12ea-9dba-00000a101bb8
  resourceVersion: "13422519"
  selfLink: /apis/app.k8s.io/v1beta1/namespaces/default/applications/simple
  uid: ee5fb0fe-4863-11ea-9dba-00000a101bb8
spec:
spec:
  componentKinds:
  - group: core.hybridapp.io
    kind: Deployable
  descriptor: {}
  selector:
    matchLabels:
      tools.hybridapp.io/application-868c6b0b-7e9a-4a96-8558-ca08bcb623c3: 868c6b0b-7e9a-4a96-8558-ca08bcb623c3
status: {}
```
