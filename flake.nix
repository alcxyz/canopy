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
        version = builtins.replaceStrings ["\n"] [""] (builtins.readFile ./VERSION);
      in {
        packages = rec {
          canopy = pkgs.buildGoModule {
            pname = "canopy";
            inherit version;
            src = self;

            # Updated automatically by CI after each release.
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
          default = canopy;
        };

        devShells.default = pkgs.mkShell {
          packages = with pkgs; [ go gopls gotools goreleaser ];
        };
      }
    );
}
