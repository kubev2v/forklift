# Esxcli plugin that wraps vmkfstools 

## build
```console
    make build
```

## install
```console
    # this step uses ansible, make sure those hosts have a public key ansible
    # could work with to connect.
    # will work if you have a vault with `VMWARE_HOST` `VMWARE_USER` `VMWARE_PASSWORD` in `~/vaults/vmware_vault.yaml`
    make install
    # or specify inline
    make install VMWARE_HOST=myhost VMWARE_USER=my_user VMWARE_PASSWORD=my_vmware_pass
``` 

## install using PowerCli
```console
    Connect-VIServer -Server server -Force -Username user -Password pass
    $vmhost = Get-VMHost -VM my-vm
    $esxcli = Get-EsxCli -VMHost $vmhost -V2
    $esxcli.software.vib.install.Invoke(@{viburl="/path/to/vmkfstools-wrapper.vib"; force=$true})
```
## invoke
```console
    esxcli vmkfstools clone -s path-to-source-vmdk -t target-lun
```

## invoke using PowerCli
```console
    $esxcli.vmkfstools.clone.Invoke(@{s="/path/to/vmdk"; t="path/to/naa.12345"})
```

## validate xcopy readiness

`xcopy-validate.sh` is a developer diagnostic tool that SSH-es into an ESXi host and checks
every setting that can silently prevent VAAI XCOPY (Full Copy) offload from working.

Requires `sshpass` on the local machine (`dnf install sshpass`).

### usage

```console
# single datastore
./xcopy-validate.sh --host <esxi-ip> --user root --password <pass> --datastore <DS-name>

# all VMFS datastores on the host
./xcopy-validate.sh --host <esxi-ip> --user root --password <pass> --all-datastores

# credentials from environment variables
export VMWARE_HOST=10.0.0.1 VMWARE_USER=root VMWARE_PASSWORD=secret
./xcopy-validate.sh --datastore MyDS

# via Makefile (reads credentials from ~/vaults/vmware_vault.yaml)
make validate-xcopy DATASTORE=MyDS
make validate-xcopy-all
```

### checks performed

| # | Check | Scope |
|---|-------|-------|
| 1 | `HardwareAcceleratedMove` = 1 | host-wide |
| 2 | `HardwareAcceleratedInit` = 1 | host-wide |
| 3 | `MaxHWTransferSize` ≥ 16384 (warns if lower, xcopy still works) | host-wide |
| 4 | VAAI Plugin Name bound to device | per device |
| 5 | Clone Status = supported | per device |
| 6 | VAAI claim rule exists for device vendor | per device |
| 7 | Path health — active/standby/dead path counts | per device |
| 8 | `failedCloneOps` / `cloneWriteOps` from vsish | per device |
| 9 | Jumbo frames end-to-end via vmkping (iSCSI only, interactive) | per device |

### example output

```
=== XCOPY Readiness Validation ===
Host: 10.0.0.1

--- Global ESXi Settings ---
[PASS] HardwareAcceleratedMove = 1
[PASS] HardwareAcceleratedInit = 1
[WARN] MaxHWTransferSize = 4096 (VMware default; xcopy works but may cause overhead ...)

--- Device-Specific Checks ---
Device:    naa.600a0980383139544924583130314c41
Vendor:    NETAPP  Model: iSCSI Disk  SATP: VMW_SATP_ALUA

[PASS] VAAI Plugin Name = VMW_VAAIP_NETAPP
[PASS] Clone Status = supported
[PASS] VAAI claim rule exists for vendor=NETAPP (plugin: VMW_VAAIP_NETAPP)
[PASS] Path health: 2 paths (2 active, 0 standby, 0 dead)
[INFO] Clone stats: cloneWriteOps=0x67392 (xcopy operations recorded)
[WARN] failedCloneOps=0x1d -- some xcopy operations failed

=== Summary [MyDS]: 4 PASS, 1 WARN, 0 FAIL — XCOPY may work but check warnings ===
```

Exit code equals the total number of FAILs across all checked datastores (0 = all clear).
