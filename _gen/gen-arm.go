//go:build armgen

// Copyright 2024+, Klaus Post/Minio Inc. See LICENSE for details.

package main

func main() {
	genArmSve()
	genArmNeon()
	genArmSveAllVl()
}
