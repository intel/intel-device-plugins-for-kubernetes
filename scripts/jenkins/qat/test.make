# Copyright 2019 Intel Corporation.
#
# SPDX-License-Identifier: Apache-2.0
#
# Makefile for QAT-DPDK jenkins.

qat: qat-checks qat-pull qat-cluster qat-plugin qat-tests

qat-checks:
	bash scripts/jenkins/qat/checks.sh

qat-pull:
	bash scripts/jenkins/qat/images-pull.sh

qat-cluster:
	bash scripts/jenkins/qat/k8s-install.sh

qat-plugin:
	bash scripts/jenkins/qat/plugin-deploy.sh

qat-tests: qat-tc-crypto qat-tc-compress

qat-tc-crypto:
	TCNAME="crypto" TCNUM=1 bash ./scripts/jenkins/qat/tc-deploy.sh

qat-tc-compress:
	TCNAME="compress" TCNUM=1 bash ./scripts/jenkins/qat/tc-deploy.sh
