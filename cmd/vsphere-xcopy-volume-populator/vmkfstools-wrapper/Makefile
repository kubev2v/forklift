build:
	./create-vib.sh

copy:
	scp vmkfstools-wrapper.vib root@ecoesxi04.lab.eng.tlv2.redhat.com:/vmfs/volumes/eco-iscsi-ds2/

install: build
	ANSIBLE_HOST_KEY_CHECKING=false \
	VMWARE_HOST=$(shell ansible-vault view ~/vaults/vmware_vault.yaml --vault-password-file ~/vaults/passfile |yq .hostname) \
	VMWARE_USER=$(shell ansible-vault view ~/vaults/vmware_vault.yaml --vault-password-file ~/vaults/passfile |yq .username) \
	VMWARE_PASSWORD=$(shell ansible-vault view ~/vaults/vmware_vault.yaml --vault-password-file ~/vaults/passfile |yq .password) \
	ansible-playbook vib-install-playbook.yaml -i esxi_hosts.yaml

inventory: 
	VMWARE_HOST=$(shell ansible-vault view ~/vaults/vmware_vault.yaml --vault-password-file ~/vaults/passfile |yq .hostname) \
	VMWARE_USER=$(shell ansible-vault view ~/vaults/vmware_vault.yaml --vault-password-file ~/vaults/passfile |yq .username) \
	VMWARE_PASSWORD=$(shell ansible-vault view ~/vaults/vmware_vault.yaml --vault-password-file ~/vaults/passfile |yq .password) \
	ansible-inventory --list -i esxi_hosts.yaml  -vvvv --become
	
