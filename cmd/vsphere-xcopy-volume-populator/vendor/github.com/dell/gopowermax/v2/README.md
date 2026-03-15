# :lock: **Important Notice**
Starting with the release of **Container Storage Modules v1.16.0**, this repository will no longer be maintained as an open source project. Future development will continue under a closed source model. This change reflects our commitment to delivering even greater value to our customers by enabling faster innovation and more deeply integrated features with the Dell storage portfolio.<br>
For existing customers using Dell’s Container Storage Modules, you will continue to receive:
* **Ongoing Support & Community Engagement**<br>
       You will continue to receive high-quality support through Dell Support and our community channels. Your experience of engaging with the Dell community remains unchanged.
* **Streamlined Deployment & Updates**<br>
        Deployment and update processes will remain consistent, ensuring a smooth and familiar experience.
* **Access to Documentation & Resources**<br>
       All documentation and related materials will remain publicly accessible, providing transparency and technical guidance.
* **Continued Access to Current Open Source Version**<br>
       The current open-source version will remain available under its existing license for those who rely on it.

Moving to a closed source model allows Dell’s development team to accelerate feature delivery and enhance integration across our Enterprise Kubernetes Storage solutions ultimately providing a more seamless and robust experience.<br>
We deeply appreciate the contributions of the open source community and remain committed to supporting our customers through this transition.<br>

For questions or access requests, please contact the maintainers via [Dell Support](https://www.dell.com/support/kbdoc/en-in/000188046/container-storage-interface-csi-drivers-and-container-storage-modules-csm-how-to-get-support).

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

