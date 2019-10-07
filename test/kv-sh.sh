#!/bin/bash

source $(dirname ${0})/util.sh

export APP_BIN="./build/vsh_linux_amd64"
export VAULT_PORT=8889
export VAULT_TOKEN="root"
export VAULT_VERSION="1.2.2"
export VAULT_ADDR="http://localhost:${VAULT_PORT}"
export VAULT_CONTAINER_NAME="vault"
export VAULT_TEST_VALUE="test"

{ # Try
start_vault ${VAULT_VERSION} ${VAULT_CONTAINER_NAME} ${VAULT_PORT}

## Setup v2 KV
vault_exec ${VAULT_CONTAINER_NAME} "vault secrets disable secret"
vault_exec ${VAULT_CONTAINER_NAME} "vault secrets enable -version=2 -path=secretkv2 kv"

vault_exec ${VAULT_CONTAINER_NAME} "vault kv put secretkv2/source/a value=${VAULT_TEST_VALUE}"
vault_exec ${VAULT_CONTAINER_NAME} "vault kv put secretkv2/source/b value=${VAULT_TEST_VALUE}"
vault_exec ${VAULT_CONTAINER_NAME} "vault kv put secretkv2/source/c/d value=${VAULT_TEST_VALUE}"
vault_exec ${VAULT_CONTAINER_NAME} "vault kv put secretkv2/source/c/e value=${VAULT_TEST_VALUE}"

vault_exec ${VAULT_CONTAINER_NAME} "vault kv put secretkv2/remove/x value=${VAULT_TEST_VALUE}"
vault_exec ${VAULT_CONTAINER_NAME} "vault kv put secretkv2/remove/y/z value=${VAULT_TEST_VALUE}"

## Setup v1 KV
vault_exec ${VAULT_CONTAINER_NAME} "vault secrets enable -version=1 -path=secretkv1 kv"

vault_exec ${VAULT_CONTAINER_NAME} "vault kv put secretkv1/source/a value=${VAULT_TEST_VALUE}"
vault_exec ${VAULT_CONTAINER_NAME} "vault kv put secretkv1/source/b value=${VAULT_TEST_VALUE}"
vault_exec ${VAULT_CONTAINER_NAME} "vault kv put secretkv1/source/c/d value=${VAULT_TEST_VALUE}"
vault_exec ${VAULT_CONTAINER_NAME} "vault kv put secretkv1/source/c/e value=${VAULT_TEST_VALUE}"

vault_exec ${VAULT_CONTAINER_NAME} "vault kv put secretkv1/remove/x value=${VAULT_TEST_VALUE}"
vault_exec ${VAULT_CONTAINER_NAME} "vault kv put secretkv1/remove/y/z value=${VAULT_TEST_VALUE}"

}
