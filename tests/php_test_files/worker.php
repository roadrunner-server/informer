<?php

declare(strict_types=1);

use Spiral\Goridge\StreamRelay;
use Spiral\RoadRunner\Worker as RoadRunner;

// the pipes relay owns STDOUT, so any diagnostic has to go to STDERR or the
// goridge frame is corrupted
ini_set('display_errors', 'stderr');

require __DIR__ . "/vendor/autoload.php";

$rr = new RoadRunner(new StreamRelay(\STDIN, \STDOUT));

// the pools exist so that the informer has worker processes to report; no
// payload is ever sent to them
while ($rr->waitPayload()) {
    $rr->respond(new \Spiral\RoadRunner\Payload(""));
}
