build:
	./create-vib.sh

copy:
	scp vmkfstools-wrapper.vib root@ocp-edge113.lab.eng.tlv2.redhat.com:/vmfs/volumes/eco-iscsi-ds2/

install: build
	ansible-playbook vib-install-playbook.yaml -i esxi_hosts.yaml
