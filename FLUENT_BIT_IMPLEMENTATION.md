# Implementazione del Sidecar Fluent Bit per il Logging in Loki

## Riepilogo delle modifiche

### 1. File creati

#### `/config/default/tool-specs.yaml`
- **Scopo**: ConfigMap con le specifiche dei tool di enumerazione
- **Contenuto**: Definizioni YAML di 8 tool (nmap, nikto, masscan, gobuster, wpscan, sqlmap, feroxbuster, dirsearch)
- **Nota**: Spostato da `internal/controller/tools_template.yml` alla directory appropriata

#### `/config/default/fluent-bit-config.yaml`
- **Scopo**: ConfigMap con la configurazione di Fluent Bit
- **Contenuto**:
  - INPUT: tail con pattern `/logs/*.log`, read_from_head=true, refresh_interval=5
  - FILTER: aggiunge label dinamici per cluster, component, app
  - OUTPUT: Loki con credenziali e label da variabili d'ambiente

#### `/config/default/loki-secret.yaml`
- **Scopo**: Secret con le credenziali e configurazioni per Loki
- **Contenuto**:
  - `loki-host`, `loki-port`, `loki-tls`, `loki-tls-verify`
  - `loki-tenant-id`, `loki-user`, `loki-password`
  - `cluster-name`

#### `/config/samples/kttack_v1alpha1_enumeration_with_loki.yaml`
- **Scopo**: Esempi di risorse Enumeration con sidecar Fluent Bit abilitato
- **Contenuto**: 3 esempi (nmap one-time, nmap scheduled, masscan)

#### `/docs/FLUENT_BIT_SIDECAR.md`
- **Scopo**: Documentazione completa del sidecar Fluent Bit
- **Contenuto**: 
  - Come funziona il sidecar
  - Configurazione richiesta
  - Deployment
  - Query in Loki
  - Troubleshooting

### 2. File modificati

#### `/internal/controller/enumeration_controller.go`
- **Aggiunto import**: `"k8s.io/apimachinery/pkg/api/resource"`
- **Aggiornato RBAC**: Aggiunto `secrets` alle risorse accessibili
- **Nuova funzione**: `buildFluentBitSidecar(enum *securityv1alpha1.Enumeration) corev1.Container`
  - Crea il container sidecar Fluent Bit
  - Monta il volume `/logs` in read-only
  - Monta la ConfigMap `fluent-bit-config` in read-only
  - Iniezione di variabili d'ambiente dal Secret `loki-credentials`
  - Variabili d'ambiente personalizzate: NAMESPACE, JOB_NAME, TOOL_TYPE
  - Resource requests/limits: 100m CPU / 64Mi memory, max 500m CPU / 256Mi memory
  
- **Modificato**: `buildPodSpec(enum)`
  - Aggiunto volume per `fluent-bit-config` ConfigMap
  - Se `debug: false`: aggiunge il sidecar Fluent Bit al pod
  - Se `debug: true`: nessun sidecar, log su stdout

#### `/config/default/kustomization.yaml`
- **Aggiunto**: Riferimenti ai 3 file di configurazione
  - `tool-specs.yaml`
  - `fluent-bit-config.yaml`
  - `loki-secret.yaml`

### 3. File eliminati

- `/internal/controller/tools_template.yml` → spostato a `/config/default/tool-specs.yaml`

## Comportamento del sidecar

### Modalità normale (debug: false)
1. Il main container redirige l'output a `/logs/job.log`
2. Fluent Bit monitora `/logs/*.log`
3. I log vengono inviati a Loki con label:
   - `job`: nome dell'Enumeration
   - `namespace`: namespace del pod
   - `tool`: tipo di tool (nmap, nikto, ecc.)
   - `cluster`: nome del cluster

### Modalità debug (debug: true)
1. Il main container scrive direttamente su stdout
2. Nessun sidecar viene aggiunto
3. I log sono disponibili via `kubectl logs`

## Label in Loki

I log vengono inviati a Loki con i seguenti label:
```
{
  job="enumeration-name",
  namespace="default",
  tool="nmap",
  cluster="k3s",
  component="job",
  app="kttack"
}
```

## Deployment

Applicare i file di configurazione prima di deployare il controller:
```bash
kubectl apply -f config/default/tool-specs.yaml
kubectl apply -f config/default/fluent-bit-config.yaml
kubectl apply -f config/default/loki-secret.yaml
```

O usare Kustomize per applicare tutto insieme:
```bash
kubectl apply -k config/default/
```

## Variabili d'ambiente nel sidecar

Il container Fluent Bit riceve le seguenti variabili d'ambiente:
- Da Secret `loki-credentials`:
  - `LOKI_HOST`, `LOKI_PORT`, `LOKI_TLS`, `LOKI_TLS_VERIFY`
  - `LOKI_TENANT_ID`, `LOKI_USER`, `LOKI_PASSWORD`
  - `CLUSTER_NAME`
- Calcolate dal pod:
  - `NAMESPACE`: namespace del pod
  - `JOB_NAME`: nome dell'Enumeration
  - `TOOL_TYPE`: tipo di tool

Queste variabili vengono utilizzate nella ConfigMap `fluent-bit-config.yaml` tramite `${VARIABLE_NAME}`.

## Resource limits del sidecar

- **Requests**: 100m CPU, 64Mi memory
- **Limits**: 500m CPU, 256Mi memory

Questi valori sono conservativi e possono essere aumentati se il sidecar processa grandi volumi di log.

## Troubleshooting

1. **I log non vengono inviati a Loki**:
   - Verificare il Secret `loki-credentials` con `kubectl describe secret loki-credentials -n kttack-system`
   - Controllare i log del sidecar: `kubectl logs <pod> -c fluent-bit-sidecar`

2. **Errore di connessione a Loki**:
   - Verificare che `loki-host` sia raggiungibile
   - Verificare certificati TLS se `loki-tls` è true
   - Verificare credenziali in Loki

3. **File `/logs` non trovato**:
   - Assicurarsi che `debug: false` nell'Enumeration
   - Verificare che il main container stia creando il file correttamente
