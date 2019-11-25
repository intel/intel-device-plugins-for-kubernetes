# Copyright 2019 Intel Corporation.
#
# SPDX-License-Identifier: Apache-2.0
#
# Makefile for QAT-DPDK jenkins.

all: plugin tests

plugin:
	bash plugin-deploy.sh

tests:
	bash tc-deploy.sh

.PHONY: all plugin tests
