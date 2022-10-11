{ pkgs ? import <nixpkgs> {} }:

pkgs.stdenv.mkDerivation rec {
	name = "smolboard";
	version = "0.0.0-1";

	nativeBuildInputs = with pkgs; [
		go
	];
}
