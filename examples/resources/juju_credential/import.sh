# Credentials can be imported by using the following pattern: 
# credentialname:cloudname:false:true
# Where false means that is not a client credential
# and true means that is a Controller credential
$ terraform import juju_credential.credential creddev:localhost:false:true
