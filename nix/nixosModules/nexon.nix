# NixOS module: declarative Nexon control-plane.
# Usage:
#   imports = [ nexon.nixosModules.nexon ];
#   services.nexon = {
#     enable = true;
#     subBaseURL = "https://vpn.example.com";
#   };
self:
{ config, lib, pkgs, ... }:
let
  cfg = config.services.nexon;
  pkg = self.packages.${pkgs.system}.nexon;
in
{
  options.services.nexon = {
    enable = lib.mkEnableOption "Nexon control-plane";

    package = lib.mkOption {
      type = lib.types.package;
      default = pkg;
      description = "The nexon package to use.";
    };

    dataDir = lib.mkOption {
      type = lib.types.path;
      default = "/var/lib/nexon";
      description = "State directory (holds the SQLite database).";
    };

    subListen = lib.mkOption {
      type = lib.types.str;
      default = ":8080";
      description = "Subscription server listen address.";
    };

    subBaseURL = lib.mkOption {
      type = lib.types.str;
      example = "https://vpn.example.com";
      description = "Public base URL used to build subscription links.";
    };

    envFile = lib.mkOption {
      type = lib.types.nullOr lib.types.path;
      default = null;
      description = "File containing extra NEXON_* env vars, loaded via EnvironmentFile.";
    };

    openFirewall = lib.mkOption {
      type = lib.types.bool;
      default = false;
      description = "Open the subscription port in the firewall.";
    };
  };

  config = lib.mkIf cfg.enable {
    systemd.services.nexon = {
      description = "Nexon control-plane";
      wantedBy = [ "multi-user.target" ];
      after = [ "network-online.target" ];
      wants = [ "network-online.target" ];
      environment = {
        NEXON_DATA_DIR = cfg.dataDir;
        NEXON_SUB_LISTEN = cfg.subListen;
        NEXON_SUB_BASE_URL = cfg.subBaseURL;
      };
      serviceConfig = {
        ExecStart = "${cfg.package}/bin/nexon serve";
        Restart = "on-failure";
        RestartSec = 3;
        StateDirectory = "nexon";
        DynamicUser = true;
        EnvironmentFile = lib.mkIf (cfg.envFile != null) cfg.envFile;
      };
    };

    networking.firewall.allowedTCPPorts = lib.mkIf cfg.openFirewall [
      (lib.toInt (lib.last (lib.splitString ":" cfg.subListen)))
    ];
  };
}
