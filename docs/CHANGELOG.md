# Changelog

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
