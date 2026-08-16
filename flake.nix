{
  description = "GopherSocial — a twitter-like full-stack social media app (Go backend + React/shadcn frontend)";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/2fcb964de67fcf60b43471c55d5d99e61a9ccb5a";
  };

  outputs = { self, nixpkgs }:
    let
      systems = [ "x86_64-linux" "aarch64-linux" ];
      forAllSystems = nixpkgs.lib.genAttrs systems;
    in
    {
      devShells = forAllSystems (system:
        let
          pkgs = nixpkgs.legacyPackages.${system};
        in
        {
          default = pkgs.mkShell {
            packages = with pkgs; [
              # Backend toolchain
              go
              gopls
              go-tools
              golangci-lint
              gotestsum

              # Frontend toolchain
              nodejs_24
              pnpm

              # Local services (no Docker required)
              postgresql_16
              redis

              # Tooling
              gnumake
              docker-compose
              air
            ];

            shellHook = ''
              echo "GopherSocial dev environment"
              echo "  - backend:  cd server && make migrate-up && air"
              echo "  - frontend: cd web && npm install && npm run dev"
              echo "  - services: make dev-services  (start Postgres + Redis, no Docker)"
              echo "              make dev-services-stop"
            '';
          };
        });
    };
}
