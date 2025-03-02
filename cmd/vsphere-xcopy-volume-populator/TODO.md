- LUN esxi FC or Iscsi luns auto discover - there may be deployments with
no auto discover set to true, on those installations adding the ESX to an initiator group is not enough for the host to see the LUN. It should be also connected to the esx

For the migration process it may be reasonable to as a customer to set auto-discover on.

