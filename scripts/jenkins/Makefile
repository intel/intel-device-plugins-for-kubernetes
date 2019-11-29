# Copyright 2019 Intel Corporation.
#
# SPDX-License-Identifier: Apache-2.0
#
# Makefile for jenkins-tests intel-device-plugins.

tests = $(shell ls -d */ | cut -f1 -d'/')

TAG?=devel

set-version:
ifneq ($(TAG),devel)
	@cd ../.. && ./scripts/set-version.sh $(TAG)
endif

$(tests):
	@make -C $@ all

tests: set-version $(tests)

info:
	@kubectl get pods
	@kubectl describe pods

logs-$(tests):
	@make -C $(subst logs-,,$@) logs

logs: info logs-$(tests)

.PHONY: set-version info logs logs-$(tests) tests $(tests)
