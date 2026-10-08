# Managed Git security acceptance

- Managed operations inspect actual Git objects even when local replacement refs exist. Catalog commits preserve the actual base tree outside the enrolled catalog, and reject durable intents that alter other paths. Replacement refs remain intact.
- Local `push.followTags=true` never expands a catalog push beyond its explicit branch refspec, including uncertain outcome retries.
- Local push signing settings and signer programs never execute during managed pushes. Repositories with legitimate signing settings remain usable and retain those settings.
- Real Git tests use temporary homes, repositories, synthetic signing markers, and a receiver advertising push certificate support. Signing variants include true, if-asked, and alternate formats.
- Existing staged and dirty files and durable retry behavior remain intact. Focused tests, mandatory PTY verification, and `make fmt check` provide evidence.
