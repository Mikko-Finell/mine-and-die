//go:generate go run mine-and-die/tools/effectsgen/cmd/effectsgen --contracts=. --registry=definitions.go --definitions=../../../config/effects/definitions.json --out=../../../client/generated/effect-contracts.ts --hash-go=effect_catalog_hash.generated.go --hash-go-pkg=contract --hash-ts=../../../client/generated/effect-contracts-hash.ts

package contract
