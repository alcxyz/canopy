{
  description = "canopy — terminal UI for multi-backend task tracking";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
        sourceVersion = builtins.replaceStrings ["\n"] [""] (builtins.readFile ./VERSION);
        revision = self.rev or self.dirtyRev or "unknown";
        developmentVersion = import ./build-version.nix {
          version = sourceVersion;
          inherit revision;
        };
        releaseVersion = import ./build-version.nix {
          version = sourceVersion;
          inherit revision;
          release = true;
        };
        mkCanopy = version: pkgs.buildGoModule {
          pname = "canopy";
          inherit version;
          src = self;

          # Update on dev when Go dependencies change; the Nix CI job reports the new value.
          vendorHash = "sha256-cpZVWNgH/SoTu117Iby4QExgP0ROPzWju6A0iUkyQ1o=";

          subPackages = [ "cmd/canopy" ];
          ldflags = [ "-s" "-w" "-X main.version=${version}" ];

          meta = with pkgs.lib; {
            description = "Terminal UI for multi-backend task tracking";
            homepage = "https://github.com/alcxyz/canopy";
            license = licenses.mit;
            mainProgram = "canopy";
          };
        };
      in {
        packages = (rec {
          canopy = mkCanopy developmentVersion;
          default = canopy;
        }) // pkgs.lib.optionalAttrs
          (builtins.match "[0-9a-f]{7,64}" revision != null)
          { release = mkCanopy releaseVersion; };

        devShells.default = pkgs.mkShell {
          packages = with pkgs; [ go gopls gotools goreleaser ];
        };
      }
    );
}
