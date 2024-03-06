# Changelog
## [0.2.0] - 2024-03-05

### Added

### Changed
- Version v2beta1 of the KafkaSchema custom resource: `spec.data.configRef` was replaced with `spec.data.schema` to simplify management of the resource (synchronization problems between K8S resources)

### Fixed
- Finalizer soft-deletes schema subject instead of removing compatibility model
- deployment.yaml: `serviceAccountName` indentation
- deployment.yaml: fixed references to values for `SCHEMA_REGISTRY_KEY` and `SCHEMA_REGISTRY_SECRET` env's

## [0.1.1] - 2023-05-17

### Added
  - `terminationProtection` (optional) to handle automatic assoiated resources if `false`
  - `autoReconciliation` (optional) to handle not reconsuming CR if `false`
  - Handle deletion and finilizers
  - Dynamic Result.Requeue policy based on parameter

### Changed
  - Updated docs
  - Move CRD to `kubernetes/crd/` folder

### Fixed 
  - Method `sendHttp` accepts nil payload

## [0.1.0] - 2023-04-04

### Added
  - Initial operator creation
  - Provision Kafka Schema with Custom Resource and Configmap
  - Add schema compatibility

### Changed

### Fixed 
