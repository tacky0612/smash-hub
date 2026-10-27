# SmashHub

[日本語](README.md) | **English**

**Even if one server shuts down, the match history survives.**

SmashHub is a **federated, non-profit, open-source** platform where *Super Smash Bros. Ultimate* players find opponents, report results, and compete on a rating ladder.

> [!IMPORTANT]
> SmashHub is an unofficial fan project and is not affiliated with Nintendo Co., Ltd. or any related company.
> "Super Smash Bros." is a registered trademark of Nintendo Co., Ltd.

> [!NOTE]
> The project is at an **early stage**. The full match flow (login → matchmaking → result reports → rating update) runs, and two instances already replicate signed match records and match players across instances using the federation protocol. Run it locally with `make up` (Docker required) and open http://localhost:8080 (Instance1) and http://localhost:8090 (Instance2). The current screens are a **provisional UI for functional testing only**; the real UI design is tracked in [#10](https://github.com/tacky0612/smash-hub/issues/10). We are looking for contributors. Most documentation is written in Japanese; English contributions and questions are welcome too.

## Why

On October 25, 2026, "Smashmate", a fan site widely used by Japanese players, shut down because running it had become too much of a burden for its operator. Sites run by a single person share the same weaknesses: when the operator stops, the service disappears and years of match history are lost with it, and all of the operational load falls on one person. And when players scatter across many replacement sites, match records and ratings get split per site (the same rating means different strength on different sites), and small sites cannot find opponents for their players.

SmashHub addresses this with a **federated architecture** similar to [Mastodon](https://joinmastodon.org/):

- Multiple independent operators run their own **instances**.
- A match result is confirmed only when **both players sign** it, so anyone can verify that a record has not been tampered with.
- Confirmed records are **replicated across instances**. When one instance closes, the records remain on the others.
- Ratings are **derived deterministically** from the records, so every instance computes the same value — there is one rating across the whole federation.
- Waiting match requests are shared between instances, so players can **match with players on other instances**, even on a small instance.
- Accounts can **move** to another instance (planned).

SmashHub is not an official successor of Smashmate. Its rating rules and formula follow Smashmate's, so its players can feel at home.

## Policies (summary)

- **No unlicensed Nintendo assets.** No game screenshots, character images, official logos, or icons in the repository or the service. Fighters and stages are shown as text only.
- **The project makes no money.** No ads, paid plans, or in-app purchases are built into the software. Other sites, commercial or not, may join the federation if they agree to the server covenant; the project takes no responsibility for their commercial activities.
- **No personal data.** Login is delegated to Discord / Google. Instances store no e-mail addresses, passwords, or external account names, only a one-way hashed identifier.

## Documentation

Start with [docs/README.md](docs/README.md) (Japanese). Key documents:

| Document | Content |
|---|---|
| [Vision](docs/vision.md) | Why we build this, and what we will not build |
| [Policy](docs/policy.md) | Trademarks, assets, non-profit, privacy |
| [Roadmap](docs/roadmap.md) | Small-start plan towards a federated system |
| [Architecture](docs/architecture/overview.md) | Overall design of the federated system |
| [Match rules](docs/domain/match-rules.md) / [Rating](docs/domain/rating.md) | Rules and rating formula |
| [Security](docs/security/README.md) | Security policy, authentication, data protection |

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) (Japanese). Please report vulnerabilities privately as described in [SECURITY.md](SECURITY.md).

## License

[GNU Affero General Public License v3.0 (AGPL-3.0)](LICENSE). It covers the whole repository, including the documentation. If you modify SmashHub and run it as an instance, you must make the modified source code available to its users.
