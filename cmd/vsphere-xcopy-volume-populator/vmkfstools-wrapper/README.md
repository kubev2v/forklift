# Esxcli plugin that wraps vmkfstools 

## build
```console
    make build
```

## install
```console
    # this step uses ansible, make sure those hosts have a public key ansible
    # could work with to connect.
    make install ESX_HOST_LIST=my-esxi-1.example.org,my-esxi-2.example.org
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
