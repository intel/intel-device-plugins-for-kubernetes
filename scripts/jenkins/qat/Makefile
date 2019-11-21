# Copyright 2019 Intel Corporation.
#
# SPDX-License-Identifier: Apache-2.0
#
# Makefile for QAT-DPDK jenkins.

all: checks plugin tests

checks:
	bash checks.sh

plugin:
	bash plugin-deploy.sh

tests:
	bash tc-deploy.sh

.PHONY: all checks plugin tests
