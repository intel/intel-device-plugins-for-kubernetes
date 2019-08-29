# Copyright 2019 Intel Corporation.
#
# SPDX-License-Identifier: Apache-2.0
#
# Makefile for QAT-DPDK jenkins.

all: plugin tests

plugin:
	@./plugin-deploy.sh

tests:
	@./tc-deploy.sh

logs:
	@./logs.sh

.PHONY: all plugin tests logs
