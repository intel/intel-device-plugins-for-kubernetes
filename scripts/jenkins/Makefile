# Copyright 2019 Intel Corporation.
#
# SPDX-License-Identifier: Apache-2.0
#
# Makefile for jenkins-tests intel-device-plugins.

tests = $(shell ls -d */ | cut -f1 -d'/')

REG?=intel/
ORG?=$(REG:/=)
TAG?=devel

set-version:
ifneq ($(ORG)$(TAG),inteldevel)
	@bash ../set-version.sh --tag $(TAG) --org $(ORG)
endif

$(tests):
	@make -C $@ all

tests: set-version $(tests)

.PHONY: set-version tests $(tests)
