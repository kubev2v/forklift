# AppArmor profile for Forklift's virt-v2v conversion and inspection pods.
#
# The container runtime default, minus its "deny mount," rule, which stops
# passt from sandboxing itself. Load on every node that may run a conversion.

#include <tunables/global>

profile forklift-virt-v2v-unshare flags=(attach_disconnected,mediate_deleted) {
  #include <abstractions/base>

  network,
  capability,
  file,
  umount,

  # What passt's sandbox needs; the default profile denies all of it.
  mount options=(rw,rslave) -> /,
  mount options=(rw,runbindable) -> /,
  mount fstype=tmpfs -> /tmp/,
  pivot_root,

  # Only mediated where kernel.apparmor_restrict_unprivileged_userns=1, Ubuntu
  # among them; harmless elsewhere.
  userns,

  deny @{PROC}/* w,
  deny @{PROC}/{[^1-9/],[^1-9/][^0-9/],[^1-9s/][^0-9y/][^0-9s/],[^1-9/][^0-9/][^0-9/][^0-9/]*}/** w,
  deny @{PROC}/sys/[^k]** w,
  deny @{PROC}/sys/kernel/{?,??,[^s][^h][^m]**} w,
  deny @{PROC}/sysrq-trigger rwklx,
  deny @{PROC}/kcore rwklx,

  deny /sys/[^f]*/** wklx,
  deny /sys/f[^s]*/** wklx,
  deny /sys/fs/[^c]*/** wklx,
  deny /sys/fs/c[^g]*/** wklx,
  deny /sys/fs/cg[^r]*/** wklx,
  deny /sys/firmware/** rwklx,
  deny /sys/devices/virtual/powercap/** rwklx,
  deny /sys/kernel/security/** rwklx,

  ptrace (trace,read,tracedby,readby) peer=forklift-virt-v2v-unshare,
}
