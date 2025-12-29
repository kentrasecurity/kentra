# ToDo List
- ~~Aggiunta Wordlist per enumeration, ddos, etc...~~
- ~~Aggiunta delle varie CRD aggiuntive~~
- ~~Aggiungere TargetIP - TargetDomain / TargetPool come CRD e campo mandatory da popolare prima dell'attacco~~
- ~~Usare MinIO come default obj storage per fetchare le wordlist da più pods~~
- Usare label (e non il nome) per gestire la configmap
- Capire come creare VPN sts
- Dare un ordine ai pod (pod eseguiti per priorità) così da avere comandi basati su altri
- Mettere nella CR StoragePools anche il prefisso s3:// o l'url
- Mettere nella CR le credenziali (così è possibile cambiare tipo di storage)
- configurare namespace per usare configmap dal namespace kentras-system
- attualmente i targetpool e storagepool funzionano solo sul namespace kentra-system
- initcontainer per spawnare un listener dopo che l'exploit è runnato (solo per la CR exploit)

