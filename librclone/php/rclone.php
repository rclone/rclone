<?php
/*
PHP interface to librclone.so, using FFI ( Foreign Function Interface )

Create an rclone object

$rc = new Rclone( __DIR__ . '/librclone.so' );

Then call rpc calls on it

    $rc->rpc( "config/listremotes", "{}" );

When finished, close it

    $rc->close();
*/

class Rclone {

    protected $rclone;
    private $out;

    public function __construct( $libshared )
    {
        $this->rclone = \FFI::cdef("
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

    public function rpc( $method, $input ): array
    {
        $this->out = $this->rclone->RcloneRPC( $method, $input );
        $response = [
            'output' => \FFI::string( $this->out->Output ),
            'status' => $this->out->Status
        ];
        $this->rclone->RcloneFreeString( $this->out->Output );
        return $response;
    }

    public function close( ): void
    {
        $this->rclone->RcloneFinalize();
    }
}
