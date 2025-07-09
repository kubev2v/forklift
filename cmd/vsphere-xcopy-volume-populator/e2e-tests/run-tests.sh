#!/bin/bash

# Exit on any error
set -e

# Run the e2e tests
echo "Running e2e tests..."
/forklift/tests.test
