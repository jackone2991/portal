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
        Schema::create('lib_chapter', function (Blueprint $table) {
            $table->id()->autoIncrement();
            $table->integer('id_book');
            $table->string('number');
            $table->string('order');
            $table->longText('description');
            $table->timestamps();
            
        });
    }
    
    /**
     * Reverse the migrations.
     */
    public function down(): void
    {
        Schema::dropIfExists('lib_chapter');
        //
    }
};
