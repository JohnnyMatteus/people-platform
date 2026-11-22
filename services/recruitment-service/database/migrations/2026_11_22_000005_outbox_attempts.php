<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

return new class extends Migration
{
    public function up(): void
    {
        Schema::table('outbox', function (Blueprint $t) {
            $t->unsignedInteger('attempts')->default(0);
            $t->text('last_error')->nullable();
        });
    }

    public function down(): void
    {
        Schema::table('outbox', function (Blueprint $t) {
            $t->dropColumn(['attempts', 'last_error']);
        });
    }
};
