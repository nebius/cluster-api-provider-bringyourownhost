#!/bin/python3
import json
import platform
import os

METADATA_FILE_PATH = '/etc/infra-k8s/host-metadata.json'
FLAGS_FILE_DIR = '/etc/byoh-agent'
FLAGS_FILE_PATH = f'{FLAGS_FILE_DIR}/byoh-agent-flags.env'

def load_metadata():
    with open(METADATA_FILE_PATH) as f:
        return json.load(f)

def create_config_dir():
    os.makedirs(FLAGS_FILE_DIR, exist_ok=True)

def construct_labels(metadata):
    common_labels = [
        "--label", f"byoh.infrastructure.cluster.x-k8s.io/type={metadata.get('node_role')}",
        "--label", f"byoh.infrastructure.cluster.x-k8s.io/cluster={metadata.get('cluster_name')}",
        "--label", f"byoh.infrastructure.cluster.x-k8s.io/hostname={platform.node()}",
    ]

    extra_labels = []
    for k, v in metadata.get('extra_labels', {}).items():
        extra_labels += ["--label", f"{k}={v}"]

    return common_labels + extra_labels

def construct_args(metadata):
    return construct_labels(metadata) + [
        "--bootstrap-kubeconfig", "/etc/infra-k8s/kubeconfig",
        "--skip-installation",
        "--namespace", f"capi-cluster-{metadata.get('cluster_name')}",
        "--metricsbindaddress", ":8081",
    ]

def write_env_file(metadata):
    # Atomically writing structured form of flags as string into '/etc/byoh-agent/byoh-agent-flags.env'
    with open(f"{FLAGS_FILE_PATH}.tmp", 'w') as f:
        f.write(f"BYOH_AGENT_FLAGS=\"{' '.join(construct_args(metadata))}\"")
    os.rename(f"{FLAGS_FILE_PATH}.tmp", FLAGS_FILE_PATH)

metadata = load_metadata()
create_config_dir()
write_env_file(metadata)