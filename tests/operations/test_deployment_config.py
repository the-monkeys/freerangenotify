"""Offline config checks: python -m unittest discover -s tests/operations -v.

Requires PyYAML; Compose checks skip explicitly when Docker CLI is unavailable.
No daemon, pulls, credentials, deliveries, or database connections are used.
"""
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest

import yaml

ROOT = Path(__file__).resolve().parents[2]
DOCKER = shutil.which("docker")
BASH = ("C:/Program Files/Git/bin/bash.exe" if os.name == "nt"
        else shutil.which("bash"))


class CollectorConfigTests(unittest.TestCase):
    def test_traces_have_bounded_buffering_and_no_payload_logging(self):
        cfg = yaml.safe_load((ROOT / "config/otel-collector.yaml").read_text())
        pipeline = cfg["service"]["pipelines"]["traces"]
        self.assertEqual(pipeline["processors"][0], "memory_limiter")
        memory = cfg["processors"]["memory_limiter"]
        self.assertGreater(memory["limit_mib"], memory["spike_limit_mib"])
        self.assertGreater(memory["spike_limit_mib"], 0)
        self.assertEqual(memory["check_interval"], "1s")
        batch = cfg["processors"]["batch"]
        self.assertGreaterEqual(batch["send_batch_max_size"], batch["send_batch_size"])
        self.assertGreater(batch["send_batch_max_size"], 0)
        self.assertEqual(pipeline["exporters"], ["otlp/tempo"])
        exporter = cfg["exporters"]["otlp/tempo"]
        self.assertGreater(exporter["sending_queue"]["queue_size"], 0)
        self.assertGreater(exporter["sending_queue"]["num_consumers"], 0)
        self.assertNotEqual(exporter["retry_on_failure"]["max_elapsed_time"], "0s")
        self.assertIn("timeout", exporter)
        self.assertNotIn("debug", cfg["exporters"])


@unittest.skipUnless(DOCKER, "Docker Compose CLI unavailable")
class ComposeConfigTests(unittest.TestCase):
    def model(self, files, project_dir=None, extra_env=None):
        env = os.environ.copy()
        env.update(GHCR_OWNER="fixture", GHCR_REPO="fixture",
                   FREERANGE_SECURITY_JWT_SECRET="fixture", JWT_SECRET="fixture",
                   FREERANGE_GRAFANA_ADMIN_PASSWORD="fixture",
                   FREERANGE_OTEL_CONFIG_DIR=str(ROOT / "config"))
        if extra_env:
            env.update(extra_env)
        args = [DOCKER, "compose", "--project-name", "ops-fixture"]
        if project_dir:
            args += ["--project-directory", str(project_dir)]
        for path in files:
            args += ["-f", str(path)]
        result = subprocess.run(args + ["config", "--no-env-resolution", "--format", "json"],
                                capture_output=True, text=True, env=env, cwd=ROOT)
        self.assertEqual(result.returncode, 0, result.stderr)
        return json.loads(result.stdout)

    def test_standalone_and_base_merges_keep_project_network_and_private_otlp(self):
        overlay = ROOT / "docker-compose.otel.yml"
        for base in (None, "docker-compose.yml", "prod/docker-compose.yaml",
                     "deploy/docker-compose.prod.yml"):
            with self.subTest(base=base):
                model = self.model(([ROOT / base] if base else []) + [overlay])
                network = model["networks"]["freerange-network"]
                self.assertFalse(network.get("external", False))
                self.assertEqual(network["name"], "ops-fixture_freerange-network")
                self.assertEqual(network.get("driver"), "bridge")
                services = model["services"]
                self.assertFalse(services["otel-collector"].get("ports"))
                for name in ("otel-collector", "tempo", "grafana"):
                    self.assertEqual(services[name]["restart"], "unless-stopped")
                self.assertGreater(int(services["otel-collector"]["mem_limit"]), 0)
                for name in ("otel-collector", "tempo", "grafana"):
                    for mount in services[name]["volumes"]:
                        if mount["type"] == "bind":
                            self.assertTrue(Path(mount["source"]).is_file())
                for name in ("tempo", "grafana"):
                    self.assertTrue(all(p["host_ip"] == "127.0.0.1" for p in services[name]["ports"]))
                if base == "docker-compose.yml":
                    ui_ports = {p["published"] for p in services["ui"]["ports"]}
                    grafana_ports = {p["published"] for p in services["grafana"]["ports"]}
                    self.assertFalse(ui_ports & grafana_ports)

    def test_standalone_external_network_is_explicit(self):
        with tempfile.TemporaryDirectory() as tmp:
            override = Path(tmp) / "external.yaml"
            for name in ("operator-existing-network", "ops-fixture_freerange-network"):
                with self.subTest(network=name):
                    override.write_text("networks:\n  freerange-network:\n    driver: !reset null\n    external: true\n    name: " + name + "\n")
                    model = self.model([ROOT / "docker-compose.otel.yml", override])
                    network = model["networks"]["freerange-network"]
                    self.assertTrue(network["external"])
                    self.assertEqual(network["name"], name)
                    self.assertNotIn("driver", network)

    def test_new_standalone_and_merged_network_declarations_match(self):
        overlay = ROOT / "docker-compose.otel.yml"
        standalone = self.model([overlay])["networks"]["freerange-network"]
        for base in ("docker-compose.yml", "prod/docker-compose.yaml", "deploy/docker-compose.prod.yml"):
            with self.subTest(base=base):
                merged = self.model([ROOT / base, overlay])["networks"]["freerange-network"]
                self.assertEqual(standalone, merged)

    def test_overlay_legacy_password_default_remains_loopback_only(self):
        model = self.model([ROOT / "docker-compose.otel.yml"],
                           extra_env={"FREERANGE_GRAFANA_ADMIN_PASSWORD": ""})
        grafana = model["services"]["grafana"]
        self.assertEqual(grafana["environment"]["GF_SECURITY_ADMIN_PASSWORD"], "admin")
        self.assertEqual(grafana["environment"]["GF_AUTH_ANONYMOUS_ENABLED"], "false")
        self.assertTrue(all(port["host_ip"] == "127.0.0.1" for port in grafana["ports"]))


@unittest.skipUnless(BASH and Path(BASH).exists() and DOCKER, "Bash/Compose unavailable")
class DeployScriptTests(unittest.TestCase):
    def invoke(self, arguments=(), settings=None, fail_config=False, fail_pull=False,
               override_content=None, fixture_env=""):
        with tempfile.TemporaryDirectory(prefix="frn ops ") as tmp:
            root = Path(tmp)
            (root / "prod").mkdir()
            (root / "config").mkdir()
            for name in ("otel-collector.yaml", "tempo.yaml", "grafana-datasources.yaml"):
                shutil.copy(ROOT / "config" / name, root / "config" / name)
            shutil.copy(ROOT / "docker-compose.otel.yml", root)
            shutil.copy(ROOT / "prod/deploy.sh", root / "prod/deploy.sh")
            shutil.copy(ROOT / "prod/docker-compose.yaml", root / "prod/docker-compose.yaml")
            (root / "prod/.env").write_text("FREERANGE_SECURITY_JWT_SECRET=fixture\nJWT_SECRET=fixture\n" + fixture_env)
            bin_dir = root / "bin"
            bin_dir.mkdir()
            # Stub only the external Docker boundary. Config uses the real Compose parser.
            (bin_dir / "docker").write_text('''#!/usr/bin/env bash
printf '%s\\n' "$*" >> "$OPS_CALLS"
for arg in "$@"; do
  if [[ "$arg" == config ]]; then
    if [[ "$OPS_FAIL_CONFIG" == 1 ]]; then exit 23; fi
    "$OPS_REAL_DOCKER" "$@" || exit $?
    "$OPS_REAL_DOCKER" "${@:1:$#-1}" --format json > "$OPS_MODEL"
    exit $?
  fi
  if [[ "$arg" == pull && "$OPS_FAIL_PULL" == 1 ]]; then exit 24; fi
done
exit 0
''')
            (bin_dir / "docker").chmod(0o755)
            env = os.environ.copy()
            # Do not inherit an operator's deployment selector into the fixture.
            for key in ("DEPLOY_OVERRIDE_FILE", "OTEL_COMPOSE_FILE", "COMPOSE_PROJECT_NAME",
                        "FREERANGE_OTEL_CONFIG_DIR", "FREERANGE_APP_ENVIRONMENT", "FREERANGE_OTEL_ENV",
                        "FREERANGE_OTEL_ENABLED", "FREERANGE_OTEL_SAMPLE_RATIO",
                        "FREERANGE_OTEL_EXPORTER_OTLP_ENDPOINT", "FREERANGE_GRAFANA_ADMIN_PASSWORD"):
                env.pop(key, None)
            env.update(PATH=str(bin_dir) + os.pathsep + env["PATH"], OPS_REAL_DOCKER=DOCKER,
                       OPS_CALLS=str(root / "calls"), OPS_MODEL=str(root / "model.json"),
                       OPS_FAIL_CONFIG=str(int(fail_config)), OPS_FAIL_PULL=str(int(fail_pull)),
                       PULL_RETRIES="2", PULL_BACKOFF="0", FREERANGE_ENABLE_OTEL="1",
                       FREERANGE_GRAFANA_ADMIN_PASSWORD="fixture")
            if settings:
                for key, value in settings.items():
                    if value is None:
                        env.pop(key, None)
                    else:
                        env[key] = value
            if override_content:
                (root / "prod/operator.yaml").write_text(override_content)
                env["DEPLOY_OVERRIDE_FILE"] = str(root / "prod/operator.yaml")
            result = subprocess.run([BASH, str(root / "prod/deploy.sh"), *arguments],
                                    capture_output=True, text=True, env=env)
            calls = (root / "calls").read_text().splitlines() if (root / "calls").exists() else []
            model = json.loads((root / "model.json").read_text()) if (root / "model.json").exists() else None
            return result, calls, model, root

    def test_check_validates_real_production_model_without_mutations(self):
        result, calls, model, root = self.invoke(["--check"])
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertTrue(calls)
        self.assertTrue(all("config" in call for call in calls), calls)
        for name in ("notification-service", "notification-worker"):
            env = model["services"][name]["environment"]
            self.assertEqual(env["FREERANGE_APP_ENVIRONMENT"], "staging")
            if name == "notification-service":
                self.assertEqual(env["FREERANGE_SERVER_PREFORK"], "false")
            self.assertEqual(env["FREERANGE_APP_DEBUG"], "false")
            self.assertEqual(env["FREERANGE_OTEL_ENV"], "production")
            self.assertEqual(env["FREERANGE_OTEL_ENABLED"], "true")
            self.assertEqual(env["FREERANGE_OTEL_EXPORTER_OTLP_ENDPOINT"], "otel-collector:4317")
        for name, source in (("otel-collector", "otel-collector.yaml"), ("tempo", "tempo.yaml"),
                             ("grafana", "grafana-datasources.yaml")):
            mounts = model["services"][name]["volumes"]
            self.assertTrue(any(Path(v["source"]) == root / "config" / source for v in mounts if v["type"] == "bind"))

    def test_selected_services_do_not_recreate_dependencies_or_prune(self):
        result, calls, _, _ = self.invoke(["notification-worker"])
        self.assertEqual(result.returncode, 0, result.stderr)
        ups = [call for call in calls if " up " in call]
        self.assertEqual(len(ups), 1)
        self.assertIn("up -d --pull never --no-deps notification-worker", ups[0])
        self.assertFalse(any(flag in " ".join(calls) for flag in ("--force-recreate", "--remove-orphans", "prune")))

    def test_bad_config_stops_before_pull_or_up(self):
        result, calls, _, _ = self.invoke(fail_config=True)
        self.assertNotEqual(result.returncode, 0)
        self.assertFalse(any(" pull" in call or " up " in call for call in calls), calls)

    def test_failed_pull_is_bounded_and_never_deploys(self):
        result, calls, _, _ = self.invoke(["notification-worker"], fail_pull=True)
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(sum(" pull" in call for call in calls), 4)
        self.assertFalse(any(" up " in call for call in calls), calls)

    def test_otel_can_be_disabled(self):
        result, calls, model, _ = self.invoke(["--check"], {"FREERANGE_ENABLE_OTEL": "0"})
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertNotIn("otel-collector", model["services"])
        self.assertEqual(model["services"]["notification-worker"]["environment"]["FREERANGE_OTEL_ENABLED"], "false")

    def test_unknown_service_and_bad_retry_settings_stop_before_mutation(self):
        for args, env in ((["does-not-exist"], {}), (["--force-recreate"], {}),
                          ([], {"PULL_RETRIES": "0"}), ([], {"PULL_BACKOFF": "invalid"})):
            with self.subTest(args=args, env=env):
                result, calls, _, _ = self.invoke(args, env)
                self.assertNotEqual(result.returncode, 0)
                self.assertFalse(any(" pull" in call or " up " in call for call in calls), calls)

    def test_dns_override_is_opt_in_and_keeps_service_networks(self):
        override = "services:\n  notification-worker:\n    dns: [192.0.2.53]\n"
        result, _, model, _ = self.invoke(["--check"], override_content=override)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(model["services"]["notification-worker"]["dns"], ["192.0.2.53"])
        self.assertNotIn("dns", model["services"]["notification-service"])
        for name in ("notification-service", "notification-worker", "otel-collector"):
            self.assertIn("freerange-network", model["services"][name]["networks"])

    def test_deployment_dotenv_overrides_trace_defaults(self):
        values = "FREERANGE_OTEL_ENABLED=false\nFREERANGE_OTEL_ENV=staging\nFREERANGE_OTEL_SAMPLE_RATIO=0.02\n"
        result, _, model, _ = self.invoke(["--check"], fixture_env=values)
        self.assertEqual(result.returncode, 0, result.stderr)
        for name in ("notification-service", "notification-worker"):
            env = model["services"][name]["environment"]
            self.assertEqual(env["FREERANGE_OTEL_ENABLED"], "false")
            self.assertEqual(env["FREERANGE_OTEL_ENV"], "staging")
            self.assertEqual(env["FREERANGE_OTEL_SAMPLE_RATIO"], "0.02")

    def test_legacy_worker_deploy_without_new_grafana_secret(self):
        for settings in ({"FREERANGE_GRAFANA_ADMIN_PASSWORD": None}, {"FREERANGE_GRAFANA_ADMIN_PASSWORD": ""}):
            with self.subTest(settings=settings):
                result, calls, model, _ = self.invoke(["notification-worker"], settings=settings)
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertEqual(model["services"]["grafana"]["environment"]["GF_SECURITY_ADMIN_PASSWORD"], "admin")
                self.assertTrue(any("up -d --pull never --no-deps notification-worker" in call for call in calls))
                self.assertFalse(any(" up " in call and call.endswith("grafana") for call in calls))

    def test_explicit_app_environment_derives_trace_tag_unless_overridden(self):
        for settings, dotenv, app, trace in (
            ({}, "FREERANGE_APP_ENVIRONMENT=staging\n", "staging", "staging"),
            ({"FREERANGE_APP_ENVIRONMENT": "development"}, "", "development", "development"),
            ({"FREERANGE_APP_ENVIRONMENT": "production"}, "", "production", "production"),
            ({}, "FREERANGE_APP_ENVIRONMENT=staging\nFREERANGE_OTEL_ENV=canary\n", "staging", "canary"),
        ):
            with self.subTest(app=app, trace=trace):
                result, _, model, _ = self.invoke(["--check"], settings=settings, fixture_env=dotenv)
                self.assertEqual(result.returncode, 0, result.stderr)
                for name in ("notification-service", "notification-worker"):
                    env = model["services"][name]["environment"]
                    self.assertEqual(env["FREERANGE_APP_ENVIRONMENT"], app)
                    self.assertEqual(env["FREERANGE_OTEL_ENV"], trace)

    def test_explicit_grafana_password_is_used_only_when_supplied(self):
        result, _, model, _ = self.invoke(["--check"], settings={"FREERANGE_GRAFANA_ADMIN_PASSWORD": None},
                                        fixture_env="FREERANGE_GRAFANA_ADMIN_PASSWORD=fixture-secret\n")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(model["services"]["grafana"]["environment"]["GF_SECURITY_ADMIN_PASSWORD"], "fixture-secret")


if __name__ == "__main__":
    unittest.main()
