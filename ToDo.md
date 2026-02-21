# ToDo List

* ~~Add wordlists for enumeration, DDoS, etc.~~
* ~~Add the various additional CRDs~~
* ~~Add TargetIP – TargetDomain / TargetPool as a CRD and make it a mandatory field to populate before the attack~~
* ~~Use MinIO as the default object storage to fetch wordlists across multiple pods~~
* Use labels (instead of names) to manage the ConfigMap
* Figure out how to create a VPN StatefulSet (STS)
* Define an execution order for pods (priority-based) so commands can depend on others
* Include the `s3://` prefix or full URL in the StoragePools CR
* Add credentials to the CR (to allow changing the storage type)
* Configure namespaces to allow using the ConfigMap from the `kentras-system` namespace
* Currently, TargetPools and StoragePools only work in the `kentra-system` namespace
* Add an initContainer to spawn a listener after the exploit has run (only for the Exploit CR)
