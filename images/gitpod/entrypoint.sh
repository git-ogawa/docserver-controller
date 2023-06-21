#!/bin/bash

set -e

function logging () {
    loglevel=$1
    message=$2
    timestamp=$(date "+%F %T.%6N")
    logmessage="[${timestamp}] ${loglevel} ${message}"
}

if [[ ! -d ~/.ssh ]]; then
    mkdir ~/.ssh
fi

if [ ${GIT_SSL_VERIFY} = "false" ]; then
    git config --global http.sslVerify false
fi

if [[ "${GIT_USERNAME}" != "" ]] && [[ "${GIT_PASSWORD}" != "" ]]; then
    comp=(${GIT_URL//\// })
    protocol=${comp[0]}
    domain=${comp[1]}
    owner=${comp[2]}
    repository=${comp[3]}

    GIT_URL="${protocol}//${GIT_USERNAME}:${GIT_PASSWORD}@${domain}/${owner}/${repository}"
fi

if [[ -f "/opt/gitpod/certs/ca.crt" ]]; then
    export GIT_SSL_CAINFO=/opt/gitpod/certs/ca.crt
fi

if [[ -d "/opt/gitpod/sshconfig" ]] && [[ -d "/opt/gitpod/privatekey" ]]; then
    cp /opt/gitpod/sshconfig/* ~/.ssh/
    cp /opt/gitpod/privatekey/* ~/.ssh/
    chmod 0400 ~/.ssh/*
fi

# Clean
rm -rf /docs/*
rm -rf /opt/gitpod/work
mkdir /opt/gitpod/work

git clone ${GIT_URL} \
    --branch ${GIT_BRANCH} \
    --depth ${GIT_DEPTH} \
    /opt/gitpod/work

cp -r /opt/gitpod/work/* /docs

logging info "Succefully completed"
