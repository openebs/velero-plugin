# Release Process
velero-plugin follows a monthly release cadence. The scope of the release is determined by contributor availability. The scope is published in the [Release Tracker Projects](https://github.com/orgs/openebs/projects).

## Release Candidate Verification Checklist

Every release has release candidate builds that are created starting from the third week into the release. These release candidate builds help to freeze the scope and maintain the quality of the release. The release candidate builds will go through:
- Platform Verification
- Regression and Feature Verification Automated tests.
- Exploratory testing by QA engineers
- Strict security scanners on the container images
- Upgrade from previous releases
- Beta testing by users on issues that they are interested in.
- Dogfooding on OpenEBS workload and e2e infrastructure clusters.

If any issues are found during the above stages, they are fixed and a new release candidate builds are generated.

Once all the above tests are completed, a main release tagged image is published.

## Release Tagging

velero-plugin is released as a container image with a versioned tag.

Before creating a release, the repo owner needs to create a separate branch from the active branch, which is `master`. Name of the branch should follow the naming convention of `v.1.9.x` if the release is for v1.9.0.

Once the release branch is created, changelog from folder `changelogs/unreleased` needs to be moved to release specific folder `changelogs/v1.9.x`, if release branch is `v1.10.x` then folder will be `changelogs/v1.10.x`.

The format of the release tag is either "Release-Name-RC1" or "Release-Name" depending on whether the tag is a release candidate or a release. (Example: v1.9.0-RC1 is a GitHub release tag for the velero-plugin release build. v1.9.0 is the release tag that is created after the release criteria are satisfied by the velero-plugin builds.)

Once the release is triggered, github actions release workflow process has to be monitored. Once github actions release workflow is passed images are pushed to docker hub and quay.io. Images can be verified by going through docker hub and quay.io. Also the images shouldn't have any high-level vulnerabilities.

Images are published at the following location:
for AMD64:
```
https://quay.io/repository/openebs/velero-plugin?tab=tags
https://hub.docker.com/r/openebs/velero-plugin/tags
```
for ARM64:
```
https://quay.io/repository/openebs/velero-plugin-arm64?tab=tags
https://hub.docker.com/r/openebs/velero-plugin-arm64/tags
```


Once a release is created, update the release description with the changelog mentioned in folder `changelog/v1.9.x`. Once the changelogs are updated in the release, the repo owner needs to create a PR to `master` with the following details:
1. update the changelog from folder `changelog/v1.9.x` to file `velero-plugin/CHANGELOG-v1.9.md`
2. If a release is an RC tag then PR should include the changes to remove the changelog from folder`changelog/v1.9.x` which are already mentioned in `velero-plugin/CHANGELOG-v1.9.md` as part of step number 1.
3. If a release is not an RC tag then
    - PR should include the changes to remove files from `changelog/v1.9.x` folder.
    - PR should update the root [CHANGELOG file](https://github.com/openebs/velero-plugin/blob/master/CHANGELOG.md) with contents of file `velero-plugin/CHANGELOG-v1.9.md`

Format of the `velero-plugin/CHANGELOG-v1.9.md` file must be as below:
```
1.9.0 / 2020-04-14
========================
* ARM build for velero-plugin, ARM image is published under openebs/velero-plugin-arm64 ([#61](https://github.com/openebs/velero-plugin/pull/61),[@akhilerm](https://github.com/akhilerm))
* Updating alpine image version for velero-plugin to 3.10.4 ([#64](https://github.com/openebs/velero-plugin/pull/64),[@mynktl](https://github.com/mynktl))
* support for local snapshot and restore(in different namespace) ([#53](https://github.com/openebs/velero-plugin/pull/53),[@mynktl](https://github.com/mynktl))
* added support for multiPartChunkSize for S3 based remote storage ([#55](https://github.com/openebs/velero-plugin/pull/55),[@mynktl](https://github.com/mynktl))
* added auto clean-up of CStor volume snapshot generated for remote backup ([#57](https://github.com/openebs/velero-plugin/pull/57),[@mynktl](https://github.com/mynktl))


1.9.0-RC2 / 2020-04-12
========================
* ARM build for velero-plugin, ARM image is published under openebs/velero-plugin-arm64 ([#61](https://github.com/openebs/velero-plugin/pull/61),[@akhilerm](https://github.com/akhilerm))
* Updating alpine image version for velero-plugin to 3.10.4 ([#64](https://github.com/openebs/velero-plugin/pull/64),[@mynktl](https://github.com/mynktl))


1.9.0-RC1 / 2020-04-08
========================
* support for local snapshot and restore(in different namespace) ([#53](https://github.com/openebs/velero-plugin/pull/53),[@mynktl](https://github.com/mynktl))
* added support for multiPartChunkSize for S3 based remote storage ([#55](https://github.com/openebs/velero-plugin/pull/55),[@mynktl](https://github.com/mynktl))
* added auto clean-up of CStor volume snapshot generated for remote backup ([#57](https://github.com/openebs/velero-plugin/pull/57),[@mynktl](https://github.com/mynktl))
```
