# Fix and enhance the use of virt-v2v in Forklift

Forklift attempts to use virt-v2v to import from VMware, but the way
it does it now does not generally work.  There are also many features
that virt-v2v provides which are not offered through Forklift.

## From VMware vCenter

The only current mode that Forklift provides is VMware vCenter (not
the more common ESXi) to a local guest on KubeVirt, so firstly we
should fix that so it works.

The only way that virt-v2v supports such conversions is by running
one of the following command lines:

```
virt-v2v \
    -ic 'vpx://root@vcenter.example.com/Datacenter/esxi?no_verify=1' \
    -ip passwordfile \
    "GUEST NAME" \
    -o kubevirt [...]
```

```
virt-v2v
    -ic 'vpx://root@vcenter.example.com/Datacenter/esxi?no_verify=1'
    -it vddk
    -io vddk-libdir=/path/to/vmware-vix-disklib-distrib
    -io vddk-thumbprint=xx:xx:xx:...
    "GUEST NAME"
    -o kubevirt [...]
```

The first version uses an HTTPS connection, which is slow but uses
entirely free software.  The second version uses VDDK, a proprietary
closed-source library, but will run much faster.

Note that virt-v2v directly connects to VMware, and this is required.

## -o kubevirt

There is a proposed enhancement to virt-v2v upstream to provide “-o
kubevirt” output mode.  In this mode, virt-v2v will write a KubeVirt
YAML file with the guest metadata.  KubeVirt must use this YAML file
to boot the guest (and not try to invent its own).

The guest disks are currently written to local files, but we intend to
change this so that we could write, for example, directly to devices
which have been attached to PVCs by the Forklift controller.  This
will require some work and coordination between KubeVirt, Forklift and
virt-v2v.

## From VMware ESXi

With the basics above fixed, we can then enhance Forklift's use of
virt-v2v to expose more features.  The first new feature that is
likely to be useful is support for importing from VMware ESXi.  This
is much more widely used than VMware vCenter.

Essentially it should work exactly the same way as imports
from vCenter above, using one of the two command lines above,
but with a small modification to the URI, replacing
`vpx://...` with `esx://root@esxi.example.com`

## From VMware OVA

Another very popular import method from VMware is to use an OVA file
(which is a kind of tar or zip file that contains the metadata and
disks).

The command line to use is:

```
virt-v2v -o ova guest.ova -o kubevirt [...]
```

A complication here is how to upload and provide the OVA file (a
regular file) to virt-v2v.

## From Xen

Although not frequently seen, virt-v2v can do conversions from Xen
over SSH.  The command line to use is:

```
virt-v2v \
    -ic 'xen+ssh://root@xen.example.com' \
    -ip passwordfile \
    guest_name \
    -o kubevirt [...]
```

## From local files

A final import method, mainly useful for testing, is to allow
importing from a plain, local disk image.  The command line to use is:

```
virt-v2v \
    -i disk \
    disk.img \
    -o kubevirt [...]
```

## Future work

Virt-v2v 2.0 offers some enhancements which could be useful to Forklift:

* Allow the copying to be offloaded to a separate process.  In this
mode virt-v2v will do the conversion and set up NBD pipelines on the
input and output sides, but defer copying to a separate process.  It's
anticipated that the Forklift controller could use this to schedule
copying across multiple virt-v2v conversions (eg. to control bandwidth
or total system load).

* Perform warm conversions.  In this mode, another process performs
the warm copy of the input guest before shutting it down and then
getting virt-v2v to do the final conversion and output.
