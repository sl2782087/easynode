"""Exercise the certificate switch on Linux with isolated files and stubbed services."""
import os
from pathlib import Path
import subprocess
import tempfile


def check(source, mode):
    with tempfile.TemporaryDirectory() as root:
        base = Path(root)
        etc = base / "panel"
        acme = base / "acme"
        live = acme / "live/easynodedeck"
        renewal = acme / "renewal"
        bins = base / "bin"
        for directory in (etc, live, renewal, bins):
            directory.mkdir(parents=True)
        old_env = "PUBLIC_URL=https://old.example\nCORE_PATH=/example\n"
        envfile = etc / "panel.env"
        envfile.write_text(old_env)
        access = etc / "initial-access"
        access.write_text("URL=https://old.example\n")
        (renewal / "easynodedeck.conf").write_text("old")
        cert = live / "cert.pem"
        cert.symlink_to("../../old-cert")
        commands = {
            "systemctl": 'echo restart >> "$TESTROOT/actions"',
            "curl": 'test "$MODE" != probe_failure',
            "certbot": 'ln -snf ../../new-cert "$LIVEDIR/cert.pem"; test "$MODE" != cert_failure',
        }
        for name, command in commands.items():
            path = bins / name
            path.write_text("#!/bin/sh\n" + command + "\n")
            path.chmod(0o700)
        script = source.replace("/etc/easynodedeck", str(etc))
        script = script.replace("/etc/letsencrypt", str(acme))
        script = script.replace("/run/easynodedeck-acme.lock", str(base / "lock"))
        script = script.replace("/opt/easynodedeck-certbot/bin/certbot", str(bins / "certbot"))
        env = dict(os.environ, MODE=mode, LIVEDIR=str(live), TESTROOT=root,
                   PATH=str(bins) + ":" + os.environ["PATH"], PUBLIC_URL="https://old.example")
        result = subprocess.run(["bash", "-c", script, "fixture", "new.example", "203.0.113.10"],
                                env=env, input="", capture_output=True, text=True)
        success = mode == "success"
        assert (result.returncode == 0) == success, (mode, result.stderr)
        assert (os.readlink(cert) == "../../new-cert") == success, mode
        if success:
            assert "PUBLIC_URL=https://new.example\n" in envfile.read_text()
            assert access.read_text() == "URL=https://new.example\n"
            assert result.stdout.strip() == "ready"
        else:
            assert envfile.read_text() == old_env
            assert access.read_text() == "URL=https://old.example\n"
        print(mode, "passed")


if __name__ == "__main__":
    source = Path(__file__).resolve().parents[1].joinpath("switch-domain.sh").read_text()
    for mode in ("success", "cert_failure", "probe_failure"):
        check(source, mode)
