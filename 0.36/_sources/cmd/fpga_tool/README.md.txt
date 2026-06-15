# Intel FPGA test tool

## Introduction

This directory contains an FPGA test tool that can be used to locate, examine and program Intel
FPGAs.

### Command line and usage

The tool has the following command line arguments:

```bash
info, fpgainfo, install, list, fmeinfo, portinfo, list-fme, list-port, pr, release, assign
```

and the following command line options:

```bash
Usage of ./fpga_tool:
  -b string
        Path to bitstream file (GBS or AOCX)
  -d string
        Path to device node (FME or Port)
  -dry-run
        Don't write/program, just validate and log
  -force
        Force overwrite operation for installing bitstreams
  -q    Quiet mode. Only errors will be reported
```