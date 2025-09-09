# pin-update.sh hack
## Prerequisites
- have installed `oc` and `jq` cli tools
- be logged into the cluster
- have installed older MTV operator version
### Example scenario

I have a MTV `2.9.1` operator installed. I want to update to MTV `2.9.2` but there is already a newer version available `2.9.3`. With default OLM update path, there is no way of updating to the wanted target version. So I will modify the `pin-update.sh` script to set the appropriate values in the `Defaults` section, mainly `MTV_VERSION` to the version I want to update to, e.g. `2.9.2`. Then, I can decide if I want the upgrade to be fully automatic, or I want to have additional control and approve the updated Install Plan for `2.9.2` manually. I want it to be automatic, so I will not remove the last part of the script, which approves the Install Plan. I can now execute the script by `./pin-update.sh`. The result is updated MTV operator to the target version `2.9.2` and not to the latest version `2.9.3`.
