<?php
/*
PHP interface to librclone.so, using FFI ( Foreign Function Interface )

Create an rclone object

$rc = new Rclone( __DIR__ . '/librclone.so' );

Then call rpc calls on it

    $rc->rcp( "config/listremotes", "{}" );

When finished, close it

    $rc->close();
*/

class Rclone {

    protected $rclone;

    function __construct( $libshared )
    {
        $this->rclone = FFI::cdef("
        struct RcloneRPCResult {
            char* Output;
            int	Status;
        };        
        extern void RcloneInitialize();
        extern void RcloneFinalize();
        extern struct RcloneRPCResult RcloneRPC(char* method, char* input);
        extern void RcloneFreeString(char* str);
        ", $libshared);
        $this->rclone->RcloneInitialize();
    }

    function rcp($method, $input ): array
    {
        $out = $this->rclone->RcloneRPC( $method, $input );
        $response = [
            'output' => FFI::string( $out->Output ),
            'status' => $out->Status
        ];
        $this->rclone->RcloneFreeString( $out->Output );
        return $response;
    }

    function close( ): void
    {
        $this->rclone->RcloneFinalize();
    }
}
