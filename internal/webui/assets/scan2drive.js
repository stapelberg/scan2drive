// vim:ts=4:sw=4:et
// Documentation references:
// https://developers.google.com/identity/sign-in/web/reference
// later superseded by: https://developers.google.com/identity/oauth2/web/guides/migration-to-gis#authorization-code-flow
// https://developers.google.com/picker/docs/reference

function httpErrorToToast(jqXHR, prefix) {
    var summary = 'HTTP ' + jqXHR.status + ': ' + jqXHR.responseText;
    Materialize.toast(prefix + ': ' + summary, 5000, 'red');
    console.log('error: prefix', prefix, ', summary', summary);
}

// https://developers.google.com/identity/oauth2/web/guides/overview says:
//
// To support a clear separation of authentication and authorization moments,
// simultaneously signing a user in to your app and to their Google Account
// while also issuing an access token is no longer supported. Previously,
// requesting an access token also signed users into their Google Account and
// returned a JWT ID token credential for user authentication.

// We use the OAuth 2.0 authorization code flow:
// https://developers.google.com/identity/oauth2/web/guides/use-code-model

var gsiClient;
var accessToken;

function initGSI() {
    const urlParams = new URLSearchParams(window.location.search);
    const errorMsg = urlParams.get('error');
    if (errorMsg !== undefined) {
	Materialize.toast(errorMsg, 5000, 'red');
    }

    accessToken = $('#user-name').attr('data-accesstoken');

    gsiClient = google.accounts.oauth2.initCodeClient({
        client_id: clientID,
        // The “profile” and “email” scope used to be automatically be
        // requested, but that is no longer the case after Google migrated to
        // Google Identity Services.
        //
        // See also https://developers.google.com/drive/api/v2/about-auth
        scope: 'https://www.googleapis.com/auth/drive.file https://www.googleapis.com/auth/userinfo.profile https://www.googleapis.com/auth/userinfo.email openid',
        ux_mode: 'redirect',
        redirect_uri: redirectURL, // from constants.js
        state: XSRFToken, // from constants.js
    });
}

function getAuthCode() {
    gsiClient.requestCode();
}

// start is called once the Google APIs were loaded
function start() {
    gapi.load('picker', function() {});

    $('#signout').click(function(ev) {
        $.ajax({
            type: 'POST',
            url: '/signout',
            success: function(result) {
                // Reload the page.
                window.location.href = window.location.origin;
            },
	    error: function(jqXHR, textStatus, errorThrown) {
		httpErrorToToast(jqXHR, 'sign out failed');
	    },
        });

        ev.preventDefault();
    });

    $('#select-drive-folder').click(function() {
        createPicker();
    });

    $('div.user').click(function() {
	var sub = $(this).data('sub');
	// TODO: show progress spinner
	$.ajax({
            type: 'POST',
            url: '/storedefaultuser',
            contentType: 'application/json',
            success: function(data, textStatus, jqXHR) {
		$('div.user').removeClass('default-user');
		$('div.user[data-sub="' + sub + '"]').addClass('default-user');
		$('#people-dialog').modal('close');
            },
            error: function(jqXHR, textStatus, errorThrown) {
		httpErrorToToast(jqXHR, 'storing default user failed');
            },
            processData: false,
            data: JSON.stringify({
		DefaultSub: sub,
            }),
	});
    });
}

function pollScan(name) {
    var user = gapi.auth2.getAuthInstance().currentUser.get();
    $.ajax({
        type: 'POST',
        url: '/scanstatus',
        contentType: 'application/json',
        success: function(data, textStatus, jqXHR) {
            $('#scan-progress-status').text(data.Status);
            console.log('result', data, 'textStatus', textStatus, 'jqXHR', jqXHR);
            if (jqXHR.status !== 200) {
                // TODO: show error message
                return;
            }
            if (data.Done) {
                $('#scan-dialog paper-spinner-lite').attr('active', null); // TODO
                $('.fixed-action-btn i,.fixed-action-btn img').text('scanner');
                $('.fixed-action-btn a').removeClass('disabled');
                $('#scan-dialog').off('iron-overlay-canceled');
                var sub = user.getBasicProfile().getId();
                $('#scan-dialog .scan-thumb').css('background', 'url("scans_dir/' + sub + '/' + name + '/thumb.png")').css('background-size', 'cover');
            } else {
                setTimeout(function() { pollScan(name); }, 500);
            }
        },
        error: function(jqXHR, textStatus, errorThrown) {
            if (jqXHR.status === 404) {
                // Scan was not yet found because the directory rescan isn’t done.
                // Retry in a little while.
                setTimeout(function() { pollScan(name); }, 500);
            } else {
                $('#scan-progress-status').text('Error: ' + errorThrown);
                setTimeout(function() { pollScan(name); }, 500);
            }
        },
        processData: false,
        data: JSON.stringify({'Name':name}),
    });
}

function renameScan(name, newSuffix) {
    var newName = name + '-' + newSuffix;

    $.ajax({
        type: 'POST',
        url: '/renamescan',
        contentType: 'application/json',
        success: function(data, textStatus, jqXHR) {
            $('#scan-form paper-input iron-icon').show();
        },
        error: function(jqXHR, textStatus, errorThrown) {
            httpErrorToToast(jqXHR, 'renaming scan failed');
        },
        processData: false,
        data: JSON.stringify({
            Name: name,
            NewName: newName,
        }),
    });
}

function scan(srcId) {
    // Only one scan can be in progress at a time.
    $('.fixed-action-btn i').text('hourglass_empty');
    $('.fixed-action-btn a').addClass('disabled');
    $('#scan-dialog').modal('open');

    $.ajax({
        type: 'POST',
        url: '/startscan/' + srcId,
        success: function(data, textStatus, jqXHR) {
            $('#scan-dialog paper-input[name="name"] div[prefix]').text(data.Name + '-');
            var renameButton = $('#scan-form paper-button');
            renameButton.click(function(ev) {
                renameScan(data.Name, $('#scan-form paper-input').val());
            });
            pollScan(data.Name);
        },
        error: function(jqXHR, textStatus, errorThrown) {
            $('#scan-dialog').modal('close');
            $('.fixed-action-btn i').text('scanner');
            $('.fixed-action-btn a').removeClass('disabled');
            httpErrorToToast(jqXHR, 'scanning failed');
        },
    });
}

// callback has “loaded”, “cancel” and “picked”
function pickerCallback(data) {
    console.log('picker callback', data);
    if (data.action !== google.picker.Action.PICKED) {
        return;
    }
    if (data.docs.length !== 1) {
        // TODO: error handling
        return;
    }
    var picked = data.docs[0];
    // TODO: show a spinner
    $.ajax({
        type: 'POST',
        url: '/storedrivefolder',
        contentType: 'application/json',
        success: function(data, textStatus, jqXHR) {
            $('#drivefolder').val(picked.name);
	    Materialize.toast('drive folder stored!', 5000, 'green');
        },
        error: function(jqXHR, textStatus, errorThrown) {
            httpErrorToToast(jqXHR, 'storing drive folder failed');
        },
        data: JSON.stringify({
            'Id': picked.id,
            'IconUrl': picked.iconUrl,
            'Url': picked.url,
            'Name': picked.name,
        }),
    });
}

function createPicker() {
    if (accessToken === undefined) {
        // The picker requires an OAuth token.
        return;
    }

    var docsView = new google.picker.DocsView()
        .setIncludeFolders(true)
        .setMimeTypes('application/vnd.google-apps.folder')
        .setMode(google.picker.DocsViewMode.LIST)
        .setOwnedByMe(true)
        .setSelectFolderEnabled(true);

    var picker = new google.picker.PickerBuilder()
        .addView(docsView)
        .setCallback(pickerCallback)
        .setOAuthToken(accessToken)
        .build();
    picker.setVisible(true);
}
