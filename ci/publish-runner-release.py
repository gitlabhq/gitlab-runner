#!/usr/bin/env python3

import argparse
import json
import os
import re
from http.client import HTTPException
from pathlib import Path
from urllib.error import HTTPError
from urllib.parse import parse_qs, quote, unquote, urlencode, urlsplit
from urllib.request import Request, urlopen


ASSET_MANIFEST = Path(__file__).with_name("runner-release-assets.json")


def request(api_url, token, method, path, payload=None):
    data = None if payload is None else json.dumps(payload).encode("utf-8")
    req = Request(api_url + path, data=data, method=method)
    # Keep the job token on the original request if urllib follows a redirect.
    req.add_unredirected_header("JOB-TOKEN", token)
    if data is not None:
        req.add_header("Content-Type", "application/json")
    try:
        with urlopen(req, timeout=30) as response:
            return json.load(response), response.headers
    except HTTPError as error:
        error.request_method = method
        raise


def ensure_release(api, releases_path, release_path, release):
    try:
        api("GET", release_path)
        return
    except HTTPError as error:
        if error.code != 404:
            raise

    try:
        api("POST", releases_path, release)
    except HTTPError as error:
        if error.code != 409:
            raise
        # Another publisher may have created the release since the lookup.
        api("GET", release_path)


def list_links(api, path):
    links = []
    page = 1
    while True:
        query = urlencode({"per_page": 100, "page": page})
        entries, headers = api("GET", f"{path}?{query}")
        links.extend(entries)

        next_page = headers.get("X-Next-Page")
        if not next_page:
            next_link = re.search(r'<([^>]+)>;\s*rel="next"', headers.get("Link", ""))
            if next_link:
                next_page = parse_qs(urlsplit(next_link[1]).query)["page"][0]
        if not next_page:
            return links
        if int(next_page) <= page:
            raise ValueError("GitLab returned an invalid next page for release links")
        page = int(next_page)


def link_exists(existing, desired, tag):
    for link in existing:
        name_matches = link["name"] == desired["name"]
        url_matches = link["url"] == desired["url"]
        direct_url = link.get("direct_asset_url") or ""
        direct_url_path = unquote(urlsplit(direct_url).path)
        direct_path = direct_url_path.partition(f"/-/releases/{tag}/downloads/")[2]
        path_matches = "/" + direct_path == desired["direct_asset_path"]
        # GitLab returns the original URL when the link has no direct download path.
        if name_matches and url_matches and (path_matches or not direct_url or direct_url == link["url"]):
            return True
        if name_matches or url_matches or path_matches:
            raise ValueError(f"Conflicting release link for {desired['name']!r}: {link['name']!r}")
    return False


def publish_links(api, path, desired_links, tag):
    existing = list_links(api, path)
    added = 0
    for desired in desired_links:
        if link_exists(existing, desired, tag):
            continue
        try:
            created, _ = api("POST", path, desired)
        except HTTPError as error:
            if error.code not in (400, 409, 422):
                raise
            # GitLab can reject a duplicate if another publisher added it first.
            existing = list_links(api, path)
            if not link_exists(existing, desired, tag):
                raise
        else:
            existing.append(created)
            added += 1
    return added


def main():
    parser = argparse.ArgumentParser(description="Publish a GitLab Runner release and its links.")
    parser.add_argument("--api-url", required=True, help="GitLab REST API base URL")
    parser.add_argument("--project-id", type=int, required=True)
    parser.add_argument("--tag", required=True)
    parser.add_argument("--description", required=True)
    parser.add_argument("--s3-base-url", required=True, help="S3 base URL for this tag")
    args = parser.parse_args()
    token = os.environ.get("CI_JOB_TOKEN")
    if not token:
        parser.error("CI_JOB_TOKEN is required")

    with ASSET_MANIFEST.open(encoding="utf-8") as manifest:
        assets = json.load(manifest)

    s3_base_url = args.s3_base_url.rstrip("/")
    release = {
        "name": args.tag,
        "tag_name": args.tag,
        "description": args.description,
    }
    links = [
        {
            "name": asset["name"],
            "url": s3_base_url + asset["s3_path"],
            "direct_asset_path": asset["direct_asset_path"],
        }
        for asset in assets
    ]
    def api(method, path, payload=None):
        return request(args.api_url.rstrip("/"), token, method, path, payload)

    releases_path = f"/projects/{args.project_id}/releases"
    release_path = f"{releases_path}/{quote(args.tag, safe='')}"
    try:
        ensure_release(api, releases_path, release_path, release)
        added = publish_links(api, f"{release_path}/assets/links", links, args.tag)
    except HTTPError as error:
        # Read past the excerpt boundary so a token crossing it can be redacted.
        secret = token.encode()
        try:
            body = error.read(500 + len(secret))
        except (OSError, HTTPException):
            body = b""
        body = body.replace(secret, b"*" * len(secret))[:500]
        detail = " ".join(body.decode("utf-8", errors="replace").split())
        detail = "".join(c if c.isprintable() else "?" for c in detail)
        message = f"GitLab API {error.request_method} {error.url} failed (HTTP {error.code})"
        if detail:
            message += f": {detail}"
        parser.exit(1, message + "\n")
    except (OSError, HTTPException, ValueError) as error:
        parser.exit(1, f"Release publishing failed: {error}\n")
    print(f"Release {args.tag} has all {len(links)} expected asset links ({added} added).")


if __name__ == "__main__":
    main()
