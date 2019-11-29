# Copyright 2019 Intel Corporation.
#
# SPDX-License-Identifier: Apache-2.0
#
# Makefile for jenkins-tests intel-device-plugins.

tests = $(shell ls -d */ | cut -f1 -d'/')

$(tests):
	@make -C $@ all

tests: $(tests)

info:
	@kubectl get pods
	@kubectl describe pods

logs-$(tests):
	@make -C $(subst logs-,,$@) logs

logs: info logs-$(tests)

.PHONY: info logs logs-$(tests) tests $(tests)
