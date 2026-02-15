#!/bin/bash

set -e

echo "Building controller..."
make docker-build

echo "Loading image into kind..."
kind load docker-image controller:latest

echo "Deploying CRDs..."
make install

echo "Deploying controller..."
make deploy

echo "Creating test CertificateBinding..."
kubectl apply -f config/samples/certauto_v1_certificatebinding.yaml

echo "Checking status..."
kubectl get certificatebindings -w