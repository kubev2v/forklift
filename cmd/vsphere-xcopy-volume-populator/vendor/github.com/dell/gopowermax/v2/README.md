# GO powermax REST library
This directory contains a lighweight Go wrapper around the Unisphere REST API

## Unit Tests
Unit Tests exist for the wrapper. These tests do not modify the array.

#### Running Unit Tests
To run these tests, from this directory, run:
```
make unit-test
```

#### Running Unit Tests With Debugging enabled
To run the tests and be able to attach debugger to the tests, run:
```
make unit-test-debug
```

Or this can be run in steps. Build the debug exec:

```
make unit-test-debug-build
```

Then to start with debugging:

```
make dlv-unit-test
```

The process will listen on port 55555 for a debugger to attach. Once the debugger is attached, the tests will start executing.

## Integration Tests
Integration Tests exist for the wrapper as well. These tests WILL MODIFY the array.

#### Pre-requisites
Before running integration tests, do the following changes in _user.env_ file in inttest folder:

* Modify the Unisphere endpoint and Symmetrix ID.

* In the file, are two variables defined:
    * username
    * password
 
   Either change those variables to match an existing user in Unisphere, or create
   a new user in Unisphere matching those credentials.

* The integration test expects certain storage objects to be present on the PowerMax array you are using for integration tests. Examine the file and modify the following declared variables with appropriate names from the PowerMax array in use.
    * Set `DefaultFCPortGroup` to an existing FC port group from the array.
    * Set `DefaultiSCSIPortGroup` to an existing iSCSI port group from the array.
    * Set `DefaultFCInitiator` to an existing FC initiatorID from the array which is not part of any host. Suffix initiatorID with its Dir:Port.
    * Set `FCInitiator1,FCInitiator2` to existing FC initiatorIDs from the array which is not part of any host.
    * Set `DefaultiSCSIInitiator` to an existing iSCSI initiatorID from the array which is not part of any host. Suffix initiatorID with its Dir:Port.
    * Set `ISCSIInitiator1,ISCSIInitiator2` to existing iSCSI initiatorIDs from the array which is not part of any host.

#### Running Integration Tests
To run these tests, from the this directory, run:

For full tests:
```
make int-test
```

For an abbreviated set of tests:
```
make short-int-test
```

