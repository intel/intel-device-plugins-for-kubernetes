# Copyright 2019 Intel Corporation.
#
# SPDX-License-Identifier: Apache-2.0
#
# Makefile for FPGA jenkins.

all: plugin tests logs info clean

plugin:
	@./plugin-deploy.sh

tests:
	@./nlb-deploy.sh

logs:
	@./logs.sh

info:
	@./info.sh

clean: 
	@./clean.sh

.PHONY: all plugin tests logs info clean