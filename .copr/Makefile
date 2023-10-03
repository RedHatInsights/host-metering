#!/usr/bin/make -f

srpm:
	# Setup development environment
	dnf install -y git make curl gzip tar rpm-build golang

	# Generate SRPM
	make rpm/srpm

	if [[ "${outdir}" != "" ]]; then \
		mv dist/rpmbuild/SRPMS/* ${outdir}/; \
	fi
