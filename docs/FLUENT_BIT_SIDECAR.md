# Fluent Bit Sidecar per il Logging in Loki

Il controller `EnumerationReconciler` include automaticamente un sidecar Fluent Bit che raccoglie i log dai job di enumerazione e li invia a Loki.

## Come funziona

Quando viene creata una risorsa `Enumeration`:

1. **Se `debug: false` (default)**:
   - Il job di enumerazione redirige l'output a `/logs/job.log`
   - Un sidecar Fluent Bit monitora il file `/logs/job.log`
   - Fluent Bit invia i log a Loki con i seguenti label:
     - `job`: nome dell'Enumeration
     - `namespace`: namespace dove gira il pod
     - `tool`: tipo di tool utilizzato (nmap, nikto, ecc.)
     - `cluster`: nome del cluster (da configurare nel Secret)

2. **Se `debug: true`**:
   - Il job di enumerazione scrive direttamente su stdout
   - Nessun sidecar viene aggiunto
   - I log sono disponibili via `kubectl logs`

## Configurazione richiesta

### Secret: `loki-credentials` in `kttack-system`

Contiene le credenziali e configurazioni per Loki:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: loki-credentials
  namespace: kttack-system
type: Opaque
stringData:
  loki-host: "loki.k3s.chungo.home"
  loki-port: "443"
  loki-tls: "true"
  loki-tls-verify: "false"
  loki-tenant-id: "1"
  loki-user: "root-user"
  loki-password: "supersecretpassword"
  cluster-name: "k3s"
```

### ConfigMap: `fluent-bit-config` in `kttack-system`

Contiene la configurazione di Fluent Bit:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: fluent-bit-config
  namespace: kttack-system
data:
  fluent-bit.conf: |
    [SERVICE]
        Flush        5
        Daemon       Off
        Log_Level    info
        Parsers_File parsers.conf

    [INPUT]
        Name              tail
        Path              /logs/*.log
        Read_from_Head    true
        Refresh_Interval  5
        Tag               kttack.job.*

    [FILTER]
        Name    modify
        Match   *
        Add     cluster ${CLUSTER_NAME}
        Add     component job
        Add     app kttack

    [OUTPUT]
        Name   loki
        Match  *
        host   ${LOKI_HOST}
        port   ${LOKI_PORT}
        tls    ${LOKI_TLS}
        tls.verify ${LOKI_TLS_VERIFY}
        tenant_id ${LOKI_TENANT_ID}
        http_user ${LOKI_USER}
        http_passwd ${LOKI_PASSWORD}
        labels job=${JOB_NAME},namespace=${NAMESPACE},tool=${TOOL_TYPE},cluster=${CLUSTER_NAME}
        label_keys job,namespace,tool,cluster
```

## Deployment

Applica i file di configurazione:

```bash
kubectl apply -f config/default/loki-secret.yaml
kubectl apply -f config/default/fluent-bit-config.yaml
```

## Esempio di utilizzo

```yaml
apiVersion: kttack.io/v1alpha1
kind: Enumeration
metadata:
  name: nmap-example
  namespace: default
spec:
  target: "192.168.1.0/24"
  tool: nmap
  debug: false  # Abilita il sidecar Fluent Bit
  periodic: false
```

## Query in Loki

Dopo l'esecuzione, puoi cercare i log con query come:

```logql
{job="nmap-example", tool="nmap", namespace="default"}
```

O filtrare per errori:

```logql
{cluster="k3s", app="kttack"} |= "error"
```

## Troubleshooting

### I log non vengono inviati a Loki

1. Verifica che il Secret `loki-credentials` esista e abbia i valori corretti:
   ```bash
   kubectl describe secret loki-credentials -n kttack-system
   ```

2. Verifica che il ConfigMap `fluent-bit-config` esista:
   ```bash
   kubectl describe configmap fluent-bit-config -n kttack-system
   ```

3. Controlla i log del sidecar Fluent Bit:
   ```bash
   kubectl logs <pod> -c fluent-bit-sidecar
   ```

### Connessione a Loki rifiutata

- Verifica che `loki-host` sia raggiungibile dal cluster
- Verifica che `loki-port` sia corretta
- Se `loki-tls` è `true`, assicurati che i certificati siano validi
- Se usi un certificato self-signed, lascia `loki-tls-verify` a `false`

### File /logs non trovato

- Assicurati che `debug: false` nel tuo Enumeration
- Verifica che il job di enumerazione stia creando il file `/logs/job.log`

## Variabili d'ambiente supportate

Il sidecar Fluent Bit riceve queste variabili d'ambiente:

- `LOKI_HOST`: Host del server Loki
- `LOKI_PORT`: Port del server Loki
- `LOKI_TLS`: Se usare TLS ("true" o "false")
- `LOKI_TLS_VERIFY`: Verificare il certificato TLS ("true" o "false")
- `LOKI_TENANT_ID`: ID del tenant in Loki
- `LOKI_USER`: Username per Loki
- `LOKI_PASSWORD`: Password per Loki
- `CLUSTER_NAME`: Nome del cluster (da Secret)
- `NAMESPACE`: Namespace del pod
- `JOB_NAME`: Nome dell'Enumeration
- `TOOL_TYPE`: Tipo di tool utilizzato
