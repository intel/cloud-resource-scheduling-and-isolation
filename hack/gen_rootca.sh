#!/bin/bash
# Copyright (C) 2025 Intel Corporation
# SPDX-License-Identifier: Apache-2.0

#set -e

# Default dir to place the Certificate
DIR_SSL_CERT="/etc/ioi/control/pki"
NAMESPACE="ioi-system"
SSLNAME=root-ca
SSLDAYS=365
COUNTRY=CN
LOCATION=Shanghai
ORGANIZATION=Intel
COMMONNAME=Root

ROOTCA_CERT=$DIR_SSL_CERT/$SSLNAME.crt
ROOTCA_KEY=$DIR_SSL_CERT/$SSLNAME.key

function print_help() {
read -t 10 -p "ATTENTION:
   You must run this script on MASTER NODE.
   Place the root CA key pair in $DIR_SSL_CERT as $DIR_SSL_CERT/root-ca.key and $DIR_SSL_CERT/root-ca.crt.
   If the root CA is not found in $DIR_SSL_CERT, a self-signed root CA will be generated.

>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
   Please click <Enter> to proceed.
>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>
   "
}

function generate_selfsigned_root_ca() {

    if [[ -f $DIR_SSL_CERT/$SSLNAME.crt ]] && [[ -f $DIR_SSL_CERT/$SSLNAME.key ]]; then
        echo "Use existing root CA in $DIR_SSL_CERT"
    else
        echo "Creating a self-signed root CA ..."
        sudo mkdir -p $DIR_SSL_CERT
        sudo rm -fr $DIR_SSL_CERT/*
        sudo chown -R $USER $DIR_SSL_CERT
        openssl genrsa -out $ROOTCA_KEY 3072 &>/dev/null
        openssl req -new -x509 -sha384 -days $SSLDAYS -key $ROOTCA_KEY -out $ROOTCA_CERT -subj "/C=${COUNTRY}/L=${LOCATION}/O=${ORGANIZATION}/CN=${COMMONNAME}"
    fi
}

function config_CA_to_secret() {
    echo "Configuring the root CA as secret 'ca-secret' ..."
    kubectl create namespace $NAMESPACE &> /dev/null
    kubectl delete secret ca-secret -n $NAMESPACE &> /dev/null
    kubectl create secret generic ca-secret -n $NAMESPACE --from-file=$SSLNAME.crt=$ROOTCA_CERT --from-file=$SSLNAME.key=$ROOTCA_KEY
}

if [[ "$1" == "skip" ]]; then
    echo "Generating root ca..."
else
    print_help
fi
generate_selfsigned_root_ca
config_CA_to_secret
