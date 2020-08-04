{ pkgs ? import <nixpkgs> {} }:

pkgs.stdenv.mkDerivation rec {
	name = "smolboard";
	version = "0.0.0-1";

	buildInputs = with pkgs; [
		libjpeg sqlite
	];

	nativeBuildInputs = with pkgs; [
		go
	];
}
