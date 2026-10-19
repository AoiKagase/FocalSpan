<?php
namespace App;
use Vendor\{Service as Shared, function process as Shared, const VALUE as Shared};

class Settings {
    #[Marker]
    public const string NAME = 'x', OTHER = 'y';
}

function Run(): void {
    new Shared();
    Shared();
    fallback();
    $unknown->process();
    Missing\process();
    include(('bootstrap.inc'));
}
