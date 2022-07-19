<?php
/*
Test program for librclone
*/

include_once ( "rclone.php" );

const REMOTE    = 'gdrive:/';
const FOLDER    = "rcloneTest";
const FILE      = "testFile.txt";

$rc = new Rclone( __DIR__ . '/librclone.so' );

$response = $rc->rpc( "config/listremotes", "{}" );
print_r( $response );

$response = $rc->rpc("operations/mkdir",
    json_encode( [
        'fs' => REMOTE,
        'remote'=> FOLDER
    ]));
print_r( $response );

$response = $rc->rpc("operations/list",
    json_encode( [
        'fs' => REMOTE,
        'remote'=> ''
    ]));
print_r( $response );

file_put_contents("./" . FILE, "Success!!!");
$response = $rc->rpc("operations/copyfile",
    json_encode( [
        'srcFs' => getcwd(),
        'srcRemote'=> FILE,
        'dstFs' => REMOTE . FOLDER,
        'dstRemote' => FILE
    ]));
print_r( $response );

$response = $rc->rpc("operations/list",
    json_encode( [
        'fs' => REMOTE . FOLDER,
        'remote'=> ''
    ]));
print_r( $response );
if ( $response['output'] ) {
    $array = @json_decode( $response['output'], true );
    if ( $response['status'] == 200 && $array['list'] ?? 0 ) {
        $valid = $array['list'][0]['Name'] == FILE ? "SUCCESS" : "FAIL";
        print_r("The test seems: " . $valid . "\n");
    }
}

$rc->close();
