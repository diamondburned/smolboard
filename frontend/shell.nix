{ pkgs ? import <nixpkgs> {} }:

pkgs.stdenv.mkDerivation rec {
	name = "wasmexp";

	buildInputs = with pkgs; [ go ];

	shellHook = ''
		go get github.com/vugu/vugu/cmd/vugugen
		alias build="vugugen -s"
	'';
}
