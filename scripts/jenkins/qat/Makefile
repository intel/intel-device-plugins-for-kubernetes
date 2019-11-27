# Copyright 2019 Intel Corporation.
#
# SPDX-License-Identifier: Apache-2.0
#
# Makefile for QAT-DPDK jenkins.

all: plugin tests

plugin:
	/bin/bash plugin-deploy.sh

tests:
	/bin/bash tc-deploy.sh

logs:
	/bin/bash logs.sh

.PHONY: all plugin tests logs
