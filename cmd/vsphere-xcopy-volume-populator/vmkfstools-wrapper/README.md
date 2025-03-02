# Esxcli plugins that wraps vmkfstools 

build:
```console
    make build
```
copy to host:
```console
    make copy
```

install:
```console
    esxcli software vib install -v /path/to/vmkfstool-wrapper.vib -f
    # to reload the esx vibs
    /etc/init.d/hostd restart
```
or using powerCli
```console
    Connect-VIServer -Server server -Force -Username user -Password pass
    $vmhost = Get-VMHost -VM my-vm
    $esxcli = Get-EsxCli -VMHost $vmhost -V2
    $esxcli.software.vib.install.Invoke(@{viburl="/path/to/vmkfstools-wrapper.vib"; force=$true})
```
invoke:
```console
    esxcli vmkfstools clone -s path-to-source-vmdk -t target-lun
```
or using powerCli
```console
    $esxcli.vmkfstools.clone.Invoke(@{s="/path/to/vmdk"; t="path/to/naa.12345"})
```
