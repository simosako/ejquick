#!/usr/bin/env python3

import argparse
import json
import posixpath
import subprocess
from pathlib import Path


def git(repository: Path, *arguments: str) -> str:
    result = subprocess.run(
        ["git", "-C", str(repository), *arguments],
        check=True,
        stdout=subprocess.PIPE,
        text=True,
    )
    return result.stdout


def source_path(attribution_path: str, relative_path: str) -> str:
    path = posixpath.normpath(posixpath.join(posixpath.dirname(attribution_path), relative_path))
    if path == ".." or path.startswith("../") or path.startswith("/"):
        raise ValueError(f"attribution reference escapes repository: {relative_path}")
    return path


def values(package: dict, singular: str, plural: str) -> list[str]:
    result = package.get(plural, [])
    if isinstance(result, str):
        result = [result]
    singular_value = package.get(singular)
    if singular_value:
        result = [singular_value, *result]
    return result


def append_package(lines: list[str], repository: Path, component: str, path: str, package: dict) -> None:
    name = package.get("Name") or package.get("Id")
    if not name:
        raise ValueError(f"attribution has no Name or Id: {component}/{path}")

    lines.extend([name, "-" * len(name)])
    for label, key in (
        ("Component", None),
        ("Version", "Version"),
        ("License", "License"),
        ("SPDX", "LicenseId"),
        ("Homepage", "Homepage"),
        ("Source", "DownloadLocation"),
    ):
        value = component if key is None else package.get(key)
        if value:
            lines.append(f"{label}: {value}")
    lines.append(f"Metadata: {component}/{path}")

    copyright_text = package.get("Copyright", "")
    if isinstance(copyright_text, list):
        copyright_text = "\n".join(copyright_text)
    copyright_file = package.get("CopyrightFile")
    if copyright_file:
        copyright_text = git(repository, "show", f"HEAD:{source_path(path, copyright_file)}").rstrip()
    if copyright_text:
        lines.extend(["", "Copyright:", copyright_text.rstrip()])

    license_files = values(package, "LicenseFile", "LicenseFiles")
    for license_file in license_files:
        license_text = git(repository, "show", f"HEAD:{source_path(path, license_file)}").rstrip()
        lines.extend(["", f"License file: {license_file}", license_text])

    if not package.get("License") and not package.get("LicenseId") and not license_files:
        raise ValueError(f"attribution has no license information: {component}/{path}")
    lines.extend(["", ""])


def collect(repository: Path, component: str, lines: list[str]) -> int:
    paths = [
        path
        for path in git(repository, "ls-tree", "-r", "--name-only", "HEAD").splitlines()
        if path.endswith("/qt_attribution.json")
    ]
    for path in paths:
        document = json.loads(git(repository, "show", f"HEAD:{path}"), strict=False)
        packages = document if isinstance(document, list) else [document]
        for package in packages:
            append_package(lines, repository, component, path, package)
    return len(paths)


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("output", type=Path)
    parser.add_argument("repositories", nargs="+", metavar="COMPONENT=PATH")
    arguments = parser.parse_args()

    lines = [
        "Qt Third-Party Notices",
        "======================",
        "",
        "Generated from the qt_attribution.json metadata and referenced license files",
        "in the exact Qt source revisions recorded in SOURCE-COMPONENTS.txt.",
        "",
    ]
    attribution_files = 0
    for specification in arguments.repositories:
        component, separator, repository = specification.partition("=")
        if not separator or not component or not repository:
            parser.error(f"invalid repository specification: {specification}")
        attribution_files += collect(Path(repository), component, lines)

    if attribution_files == 0:
        raise RuntimeError("no qt_attribution.json files were found")
    arguments.output.write_text("\n".join(lines), encoding="utf-8")


if __name__ == "__main__":
    main()
