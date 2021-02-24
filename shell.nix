{ pkgs ? import <nixpkgs> { } }:

with pkgs;

mkShell { buildInputs = [ go go-tools goimports gopls ]; }
