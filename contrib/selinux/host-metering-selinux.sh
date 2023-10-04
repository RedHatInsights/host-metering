#!/bin/sh -e

DIRNAME=`dirname $0`
cd $DIRNAME
USAGE="$0 [ --update ]"
if [ `id -u` != 0 ]; then
echo 'You must be root to run this script'
exit 1
fi

if [ $# -eq 1 ]; then
	if [ "$1" = "--update" ] ; then
		time=`ls -l --time-style="+%x %X" host-metering.te | awk '{ printf "%s %s", $6, $7 }'`
		rules=`ausearch --start $time -m avc --raw -se host-metering`
		if [ x"$rules" != "x" ] ; then
			echo "Found avc's to update policy with"
			echo -e "$rules" | audit2allow -R
			echo "Do you want these changes added to policy [y/n]?"
			read ANS
			if [ "$ANS" = "y" -o "$ANS" = "Y" ] ; then
				echo "Updating policy"
				echo -e "$rules" | audit2allow -R >> host-metering.te
				# Fall though and rebuild policy
			else
				exit 0
			fi
		else
			echo "No new avcs found"
			exit 0
		fi
	else
		echo -e $USAGE
		exit 1
	fi
elif [ $# -ge 2 ] ; then
	echo -e $USAGE
	exit 1
fi

echo "Building and Loading Policy"
set -x
make -f /usr/share/selinux/devel/Makefile host-metering.pp || exit
/usr/sbin/semodule -i host-metering.pp

# Generate a man page off the installed module
sepolicy manpage -p . -d hostmetering_t
# Fixing the file context on /usr/bin/host-metering
/sbin/restorecon -F -R -v /usr/bin/host-metering
# Fixing the file context on /usr/lib/systemd/system/host-metering.service
/sbin/restorecon -F -R -v /usr/lib/systemd/system/host-metering.service
# Fixing the file context on /var/run/host-metering
/sbin/restorecon -F -R -v /var/run/host-metering
# Generate a rpm package for the newly generated policy

pwd=$(pwd)
