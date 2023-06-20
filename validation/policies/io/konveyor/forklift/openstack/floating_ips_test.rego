package io.konveyor.forklift.openstack

test_without_floating_ips {
	mock_vm := {
		"name": "test",
		"addresses": {
      "network1": [
        {
          "OS-EXT-IPS:type": "fixed",
        },
        {
          "OS-EXT-IPS:type": "fixed",
        },
      ],
      "network2": [
        {
          "OS-EXT-IPS:type": "fixed",
        },
        {
          "OS-EXT-IPS:type": "fixed",
        }
      ]
		},
	}
	results := concerns with input as mock_vm
	count(results) == 0
}

test_with_floating_ips {
	mock_vm := {
		"name": "test",
		"addresses": {
      "network1": [
        {
          "OS-EXT-IPS:type": "fixed",
        },
        {
          "OS-EXT-IPS:type": "fixed",
        },
        {
          "OS-EXT-IPS:type": "floating",
        }
      ],
      "network2": [
        {
          "OS-EXT-IPS:type": "fixed",
        },
        {
          "OS-EXT-IPS:type": "fixed",
        }
      ]
		},
	}
	results := concerns with input as mock_vm
	count(results) == 1
}
