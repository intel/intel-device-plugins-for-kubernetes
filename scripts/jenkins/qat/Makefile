# Copyright 2019 Intel Corporation.
#
# SPDX-License-Identifier: Apache-2.0
#
# Makefile for QAT-DPDK jenkins.

qat: qat-checks qat-cluster qat-pull qat-plugin qat-tests

qat-checks:
	bash checks.sh

qat-pull:
	bash images-pull.sh

qat-cluster:
	bash k8s-install.sh

qat-plugin:
	bash plugin-deploy.sh

qat-tests: qat-tc-crypto qat-tc-compress

qat-tc-crypto:
	TCNAME="crypto" TCNUM=1 bash tc-deploy.sh

qat-tc-compress:
	TCNAME="compress" TCNUM=1 bash tc-deploy.sh

.PHONY: qat qat-checks qat-pull qat-cluster qat-plugin qat-tests qat-tc-crypto qat-tc-compress
