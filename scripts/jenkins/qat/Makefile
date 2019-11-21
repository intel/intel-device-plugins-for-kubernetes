# Copyright 2019 Intel Corporation.
#
# SPDX-License-Identifier: Apache-2.0
#
# Makefile for QAT-DPDK jenkins.

qat: qat-checks qat-pull qat-plugin qat-tests

qat-checks:
	bash checks.sh

qat-pull:
	bash images-pull.sh

qat-plugin:
	bash plugin-deploy.sh

qat-tests:
	bash tc-deploy.sh

.PHONY: qat qat-checks qat-pull qat-plugin qat-tests
