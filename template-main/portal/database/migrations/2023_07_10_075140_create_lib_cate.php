<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

return new class extends Migration
{
    /**
     * Run the migrations.
     */
    public function up(): void
    {
        Schema::create('lib_cate', function (Blueprint $table) {
            $table->id()->autoIncrement();
            $table->integer('id_type');
            $table->string('name_vn');
            $table->string('name_en');
            $table->string('status');
            $table->longText('description');
            $table->timestamps();
            
        });
    }

    /**
     * Reverse the migrations.
     */
    public function down(): void
    {
        Schema::dropIfExists('lib_cate');
        //
    }
};
