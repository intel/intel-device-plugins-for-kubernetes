ARCH_LIBDIR ?= /lib/x86_64-linux-gnu

ifeq ($(DEBUG),1)
	GRAMINE_LOG_LEVEL = debug
else
	GRAMINE_LOG_LEVEL = error
endif

.PHONY: all
all: stress-ng.manifest stress-ng-edmm.manifest
ifeq ($(SGX),1)
all: stress-ng.manifest.sgx stress-ng.sig stress-ng-edmm.manifest.sgx stress-ng-edmm.sig
endif

stress-ng.manifest: stress-ng.manifest.template
	gramine-manifest \
		-Dlog_level=$(GRAMINE_LOG_LEVEL) \
		-Dedmm='false' \
		-Denclave_size=128M \
		-Dexecdir=$(shell dirname $(shell which stress-ng)) \
		-Darch_libdir=$(ARCH_LIBDIR) \
		$< >$@

stress-ng.manifest.sgx: stress-ng.manifest
	gramine-sgx-sign \
		--manifest stress-ng.manifest \
		--output $@

stress-ng.sig: stress-ng.manifest.sgx

stress-ng-edmm.manifest: stress-ng.manifest.template
	gramine-manifest \
		-Dlog_level=$(GRAMINE_LOG_LEVEL) \
		-Dedmm='true' \
		-Denclave_size=128G \
		-Dexecdir=$(shell dirname $(shell which stress-ng)) \
		-Darch_libdir=$(ARCH_LIBDIR) \
		$< >$@

stress-ng-edmm.manifest.sgx: stress-ng.manifest
	gramine-sgx-sign \
		--manifest stress-ng-edmm.manifest \
		--output $@

stress-ng-edmm.sig: stress-ng-edmm.manifest.sgx

.PHONY: clean
clean:
	$(RM) *.manifest *.manifest.sgx *.token *.sig OUTPUT

.PHONY: distclean
distclean: clean
