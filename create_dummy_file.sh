#!/bin/bash

# Create a 500MB dummy file for testing file transfers
dd if=/dev/zero of=dummy.bin bs=1M count=500

echo "Created dummy.bin (500MB) in current directory"